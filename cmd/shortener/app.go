package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/config/db"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type app struct {
	server  *http.Server
	service *service.Service
}

func newApp(cfg *config.Config) *app {
	pgxPool, err := db.New(cfg.DataBase)
	if err != nil {
		logger.Log.Sugar().Infow("running without connection to database", "cause", err)
	}

	if pgxPool != nil {
		if err := applyMigrations(cfg.DataBase); err != nil {
			logger.Log.Sugar().Fatalw("failed to apply migrations", "error", err)
		}
	}

	storage, err := initStorage(cfg, pgxPool)
	if err != nil {
		logger.Log.Sugar().Fatalw("failed to init storage", "error", err)
	}

	s := service.New(storage)
	h := handler.New(cfg.ShortURLBaseAddr, s)

	router := initChiRouter(cfg, h)
	server := initServer(cfg, router)

	return &app{
		server:  server,
		service: s,
	}
}

func (a *app) start() {
	logger.Log.Sugar().Infow("starting server", "address", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Log.Sugar().Fatalw("server starting failed", "error", err)
	}
}

func (a *app) shutdown() {
	a.service.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		logger.Log.Sugar().Errorw("server gracefull shutdown failed", "error", err)
		return
	}

	logger.Log.Sugar().Infow("shutdown successfully completed")
}

func applyMigrations(cfg *db.Config) error {
	logger.Log.Sugar().Info("applying migrations...")

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("create db driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "pgx", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("up migrations: %w", err)
	}

	logger.Log.Sugar().Info("migrations successfully applied")
	return nil
}

func initStorage(cfg *config.Config, db *pgxpool.Pool) (service.Repository, error) {
	switch {
	case db != nil:
		return repository.NewPostgres(db), nil
	case cfg.FileStoragePath != "":
		return repository.NewFile(cfg.FileStoragePath)
	default:
		return repository.NewMemory(), nil
	}
}

func initServer(conf *config.Config, router http.Handler) *http.Server {
	return &http.Server{
		Addr:         conf.ServerAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
}
