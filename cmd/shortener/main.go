package main

import (
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
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	conf, err := config.New()
	if err != nil {
		log.Fatalf("config init error: %s", err)
	}

	if err := logger.Initialize(conf.LogLevel); err != nil {
		log.Fatalf("logger init error: %s", err)
	}

	db, err := db.NewPG(&conf.DataBase)
	if err != nil {
		logger.Log.Sugar().Infow("running without connection to database", "cause", err)
	}

	var r service.Repository
	switch {
	case db != nil:
		r = repository.NewPostgres(db)
	case conf.FileStoragePath != "":
		if r, err = repository.NewFileStorage(conf.FileStoragePath); err != nil {
			logger.Log.Sugar().Fatalw("failed to init file repository", "error", err)
		}
	default:
		r = repository.NewMemory()
	}
	defer r.Close()

	s := service.New(r)
	h := handler.New(conf, s)
	server := initServer(conf, newChiMux(h))

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go start(server)

	<-shutdownSignal

	logger.Log.Sugar().Info("received shutdown signal")
	shutdown(server)
}
