package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/config/db"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("config init error: %s", err)
	}

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		log.Fatalf("logger init error: %s", err)
	}

	dbHandle, err := db.New(cfg.DataBase)
	if err != nil {
		logger.Log.Sugar().Infow("running without connection to database", "cause", err)
	} else {
		if err := runMigrations(cfg.DataBase); err != nil {
			logger.Log.Sugar().Fatalw("failed to run migrations", "error", err)
		}
	}

	r, err := initRepository(cfg, dbHandle)
	if err != nil {
		logger.Log.Sugar().Fatalw("failed to init repository", "error", err)
	}
	defer r.Close()

	s := service.New(r)
	h := handler.New(cfg, s)
	server := initServer(cfg, newChiMux(h))

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go start(server)

	<-shutdownSignal

	logger.Log.Sugar().Infow("received shutdown signal")
	shutdown(server)
}

func initRepository(cfg *config.Config, db *sql.DB) (service.Repository, error) {
	switch {
	case db != nil:
		return repository.NewPostgres(db), nil
	case cfg.FileStoragePath != "":
		r, err := repository.NewFile(cfg.FileStoragePath)
		return r, err
	default:
		return repository.NewMemory(), nil
	}
}

func runMigrations(cfg *db.Config) error {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "pgx", driver)
	if err != nil {
		return err
	}
	defer m.Close()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	logger.Log.Sugar().Info("db migrations applied")
	return nil
}
