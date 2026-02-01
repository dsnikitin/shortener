package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/dsnikitin/shortener/internal/auditor"
	"github.com/dsnikitin/shortener/internal/auditor/consumer"
	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/config/audit"
	"github.com/dsnikitin/shortener/internal/config/db"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
)

type app struct {
	server  *http.Server
	service *service.Service
	auditor *auditor.Auditor
}

func newApp(cfg *config.Config) *app {
	pgxPool, err := db.New(cfg.DataBase)
	if err != nil {
		logger.Log.Infow("Running without connection to database", "cause", err)
	}

	if pgxPool != nil {
		if err := applyMigrations(cfg.DataBase); err != nil {
			logger.Log.Fatalw("Failed to apply migrations", "error", err)
		}
	}

	storage, err := initStorage(cfg, pgxPool)
	if err != nil {
		logger.Log.Fatalw("Failed to init storage", "error", err)
	}

	auditor, err := initAuditor(cfg.Audit)
	if err != nil {
		logger.Log.Fatalw("Failed to init auditor", "error", err)
	}

	s := service.New(storage)
	h := handler.New(cfg.ShortURLBaseAddr, s, auditor)

	router := initChiRouter(cfg, h)
	server := initServer(cfg, router)

	return &app{
		server:  server,
		service: s,
		auditor: auditor,
	}
}

func (a *app) start() {
	logger.Log.Infow("Starting server", "address", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Log.Fatalw("Server starting failed", "error", err)
	}
}

func (a *app) shutdown() {
	a.service.Stop()
	a.auditor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		logger.Log.Errorw("Server gracefull shutdown failed", "error", err)
		return
	}

	logger.Log.Infow("Shutdown successfully completed")
}

func applyMigrations(cfg *db.Config) error {
	logger.Log.Info("Applying migrations...")

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

	logger.Log.Info("Migrations successfully applied")
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
		ReadTimeout:  40 * time.Second,
		WriteTimeout: 40 * time.Second,
	}
}

func initAuditor(cfg *audit.Config) (*auditor.Auditor, error) {
	var consumers []auditor.Consumer

	if cfg.FilePath != "" {
		fileConsumer, err := consumer.NewFile(cfg.FileConsumerID, cfg.FilePath, cfg.EventsLimit)
		if err != nil {
			return nil, fmt.Errorf("init file audit consumer: %w", err)
		}

		consumers = append(consumers, fileConsumer)
	}

	if cfg.URL != "" {
		remoteConsumer, err := consumer.NewRemote(cfg.RemoteConsumerID, cfg.URL, cfg.EventsLimit)
		if err != nil {
			return nil, fmt.Errorf("init remote audit consumer: %w", err)
		}

		if err := remoteConsumer.HealthCheck(); err != nil {
			return nil, fmt.Errorf("healthcheck remote audit consumer: %w", err)
		}

		consumers = append(consumers, remoteConsumer)
	}

	if len(consumers) == 0 {
		logger.Log.Info("Starting without audit consumers")
	}

	return auditor.New(cfg.EventsLimit, consumers...), nil
}
