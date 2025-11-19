package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
)

func main() {
	conf, err := config.New()
	if err != nil {
		log.Fatalf("config init error: %s", err)
	}

	if err := logger.Initialize(conf.LogLevel); err != nil {
		log.Fatalf("logger init error: %s", err)
	}

	repo, err := repository.New(conf.FileStoragePath)
	if err != nil {
		logger.Log.Sugar().Fatalw("failed to init repository", "error", err)
	}
	defer repo.Close()

	service := service.New(repo)
	handler := handler.New(conf, service)
	server := initServer(conf, newChiMux(handler))

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go start(server)

	<-shutdownSignal

	logger.Log.Sugar().Info("received shutdown signal")
	shutdown(server)
}
