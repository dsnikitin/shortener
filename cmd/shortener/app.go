package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/dsnikitin/shortener/api/proto"
	"github.com/dsnikitin/shortener/internal/auditor"
	"github.com/dsnikitin/shortener/internal/auditor/consumer"
	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/config/audit"
	"github.com/dsnikitin/shortener/internal/config/db"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/middleware"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
	"github.com/dsnikitin/shortener/internal/service/deleter"
)

type app struct {
	cfg        *config.Config
	httpServer *http.Server
	grpcServer *grpc.Server
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
	httpHandler := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, auditor)
	grpcHandler := handler.NewGRPCHandler(cfg.ShortURLBaseAddr, s, auditor)

	tlsCfg, err := initTLSConfig(cfg)
	if err != nil {
		logger.Log.Fatalw("Failed to init tls config", "error", err)
	}

	return &app{
		cfg:        cfg,
		httpServer: initHTTPServer(cfg, initChiRouter(cfg, httpHandler), tlsCfg),
		grpcServer: initGRPCServer(cfg, grpcHandler, tlsCfg),
		service:    s,
		urlDeleter: urlDeleter,
		auditor:    auditor,
	}
}

func (a *app) startHTTPServer() {
	if !a.cfg.EnableHTTPS {
		logger.Log.Infow("Starting HTTP server", "address", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatalw("HTTP server starting failed", "error", err)
		}
		return
	}

	// иначе запускаем https-сервер
	logger.Log.Infow("Starting HTTP server with TLS", "address", a.httpServer.Addr)
	err := a.httpServer.ListenAndServeTLS("", "")
	if err != nil && err != http.ErrServerClosed {
		logger.Log.Fatalw("HTTP server starting failed", "error", err)
	}
}

func (a *app) startGRPCServer() {
	if !a.cfg.EnableHTTPS {
		logger.Log.Infow("Starting gRPC server", "address", a.cfg.GRPCServerAddr)
	} else {
		logger.Log.Infow("Starting gRPC server with TLS", "address", a.cfg.GRPCServerAddr)
	}

	listener, err := net.Listen("tcp", a.cfg.GRPCServerAddr)
	if err != nil {
		logger.Log.Fatalw("Listener initializition failed", "error", err)
	}

	if err = a.grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
		logger.Log.Fatalw("gRPC server starting error", "error", err)
	}
}

func (a *app) shutdown() {
	components := []struct {
		name    string
		timeout time.Duration
		stopFn  func(ctx context.Context) error
	}{
		{
			// сначала останавливаем серверы, чтобы не принимать новые запросы
			name:    "HTTP Server",
			stopFn:  a.httpServer.Shutdown,
			timeout: time.Second * 30,
		},
		{
			name:    "gRPC Server",
			stopFn:  a.stopGRPCServerWithContext,
			timeout: time.Second * 30,
		},
		{
			// останавливаем перед сервисом, ему нужен еще не закрытый репозиторий
			name:    "URL deleter",
			stopFn:  a.urlDeleter.Stop,
			timeout: time.Second * 30,
		},
		{
			// останавливаем сервис, закрываем репозиторий
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

func initTLSConfig(cfg *config.Config) (*tls.Config, error) {
	if !cfg.EnableHTTPS {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFilePath, cfg.PrivateKeyFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "load tls certificate")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return tlsCfg, nil
}

func initHTTPServer(cfg *config.Config, h http.Handler, tlsCfg *tls.Config) *http.Server {
	server := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      h,
		ReadTimeout:  40 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if tlsCfg != nil {
		server.TLSConfig = tlsCfg
	}

	return server
}

func initGRPCServer(cfg *config.Config, h pb.ShortenerServiceServer, tlsCfg *tls.Config) *grpc.Server {
	var server *grpc.Server
	if tlsCfg != nil {
		server = grpc.NewServer(
			grpc.Creds(credentials.NewTLS(tlsCfg)),
			grpc.UnaryInterceptor(middleware.AuthInterceptor(cfg.JWTSigningKey)),
		)
	} else {
		server = grpc.NewServer(
			grpc.UnaryInterceptor(middleware.AuthInterceptor(cfg.JWTSigningKey)),
		)
	}

	pb.RegisterShortenerServiceServer(server, h)
	return server
}

func (a *app) stopGRPCServerWithContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		select {
		case <-done:
		default:
			return ctx.Err()
		}
	}

	return nil
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
