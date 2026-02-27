package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/auditor"
	"github.com/dsnikitin/shortener/internal/auditor/consumer"
	"github.com/dsnikitin/shortener/internal/certx"
	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/config/audit"
	"github.com/dsnikitin/shortener/internal/config/db"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
	"github.com/dsnikitin/shortener/internal/service/deleter"
)

type app struct {
	cfg        *config.Config
	server     *http.Server
	service    *service.Service
	urlDeleter *deleter.Deleter
	auditor    *auditor.Auditor
}

func newApp(cfg *config.Config) *app {
	pgxPool, err := db.New(cfg.DataBase)
	if err != nil {
		logger.Log.Infow("Running without connection to database", "cause", err)
	}

	if pgxPool != nil {
		if err = applyMigrations(cfg.DataBase); err != nil {
			logger.Log.Fatalw("Failed to apply migrations", "error", err)
		}
	}

	r, err := initRepo(cfg, pgxPool)
	if err != nil {
		logger.Log.Fatalw("Failed to init repo", "error", err)
	}

	urlDeleter := deleter.New(r)

	auditor, err := initAuditor(cfg.Audit)
	if err != nil {
		logger.Log.Fatalw("Failed to init auditor", "error", err)
	}

	s := service.New(r, urlDeleter)
	h := handler.New(cfg.ShortURLBaseAddr, s, auditor)

	router := initChiRouter(cfg, h)
	server := initServer(cfg, router)

	return &app{
		cfg:        cfg,
		server:     server,
		service:    s,
		urlDeleter: urlDeleter,
		auditor:    auditor,
	}
}

func (a *app) start() {
	if !a.cfg.EnableHTTPS {
		logger.Log.Infow("Starting HTTP server", "address", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatalw("HTTP server starting failed", "error", err)
		}
		return
	}

	// иначе запускаем https-сервер
	if err := a.ensureValidCert(); err != nil {
		logger.Log.Fatalw("Failed to ensure valid cert", "error", err)
	}

	logger.Log.Infow("Starting HTTPS server", "address", a.server.Addr)
	err := a.server.ListenAndServeTLS(a.cfg.CertFilePath, a.cfg.PrivateKeyFilePath)
	if err != nil && err != http.ErrServerClosed {
		logger.Log.Fatalw("HTTPS server starting failed", "error", err)
	}
}

func (a *app) shutdown() {
	components := []struct {
		name    string
		timeout time.Duration
		stopFn  func(ctx context.Context) error
	}{
		{
			// останавливаем первым, чтобы не принимать новые запросы
			name:    "Server",
			stopFn:  a.server.Shutdown,
			timeout: time.Second * 30,
		},
		{
			// останавливаем вторым, ему нужен еще не закрытый репозиторий
			name:    "URL deleter",
			stopFn:  a.urlDeleter.Stop,
			timeout: time.Second * 30,
		},
		{
			// останавливаем третьим, закрываем репозиторий
			name:    "Service",
			stopFn:  a.service.Stop,
			timeout: time.Second * 30,
		},
		{
			// останавливаем последним, он записывает свершившиеся события
			name:    "Auditor",
			stopFn:  a.auditor.Stop,
			timeout: time.Second * 30,
		},
	}

	for _, c := range components {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)

		logger.Log.Infof("Stopping %s...", c.name)
		if err := c.stopFn(ctx); err != nil {
			logger.Log.Errorw(fmt.Sprintf("Failed to stop %s", c.name), "error", err)
		} else {
			logger.Log.Infof("%s stopped gracefully", c.name)
		}

		cancel()
	}
}

func applyMigrations(cfg *db.Config) error {
	logger.Log.Info("Applying migrations...")

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return errors.Wrap(err, "open db")
	}
	defer db.Close()

	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return errors.Wrap(err, "create db driver")
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "pgx", driver)
	if err != nil {
		return errors.Wrap(err, "create migrate instance")
	}
	defer m.Close()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return errors.Wrap(err, "up migrations")
	}

	logger.Log.Info("Migrations successfully applied")
	return nil
}

func initRepo(cfg *config.Config, db *pgxpool.Pool) (service.Repository, error) {
	switch {
	case db != nil:
		return repository.NewPostgres(db), nil
	case cfg.FileStoragePath != "":
		return repository.NewFile(cfg.FileStoragePath)
	default:
		return repository.NewMemory(), nil
	}
}

func initServer(cfg *config.Config, router http.Handler) *http.Server {
	server := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  40 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.EnableHTTPS {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server.TLSConfig = tlsCfg
	}

	return server
}

func initAuditor(cfg *audit.Config) (*auditor.Auditor, error) {
	var consumers []auditor.Consumer

	if cfg.FilePath != "" {
		fileConsumer, err := consumer.NewFile(cfg.FileConsumerID, cfg.FilePath, cfg.EventsLimit)
		if err != nil {
			return nil, errors.Wrap(err, "init file audit consumer")
		}

		logger.Log.Infof("Consumer %s started", fileConsumer.GetID())
		consumers = append(consumers, fileConsumer)
	}

	if cfg.URL != "" {
		remoteConsumer, err := consumer.NewRemote(cfg.RemoteConsumerID, cfg.URL, cfg.EventsLimit)
		if err != nil {
			return nil, errors.Wrap(err, "init remote audit consumer")
		}

		logger.Log.Infof("Consumer %s started", remoteConsumer.GetID())
		consumers = append(consumers, remoteConsumer)
	}

	if len(consumers) == 0 {
		logger.Log.Info("Starting without audit consumers")
	}

	return auditor.New(cfg.EventsLimit, consumers...), nil
}

func (a *app) ensureValidCert() error {
	err := certx.CheckCert(a.cfg.CertFilePath, a.cfg.PrivateKeyFilePath)
	if err == nil {
		return nil
	}

	// в dev режиме генерируем самоподписанный сертификат и приватный ключ
	if a.cfg.IsDevelop &&
		(errors.Is(err, certx.ErrKeyFileNotFound) ||
			errors.Is(err, certx.ErrCertFileNotFound) ||
			errors.Is(err, certx.ErrCertNotYetValid) ||
			errors.Is(err, certx.ErrCertExpired)) {

		err = certx.GenerateSelfSignedCert(a.cfg.CertFilePath, a.cfg.PrivateKeyFilePath)
		return errors.Wrap(err, "generate self-signed certificate")
	}

	return errors.Wrap(err, "check certificate")
}
