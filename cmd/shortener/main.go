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
	conf := config.NewFromArgs()

	if err := logger.Initialize(conf.LogLevel); err != nil {
		log.Fatalf("init logger error: %s", err)
	}

	file, err := os.OpenFile(conf.FileStoragePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Log.Sugar().Fatalw("open storage file", "error", err)
	}
	defer file.Close()

	repo, _ := repository.New(file)
	service := service.New(repo)
	handler := handler.New(conf, service)
	server := initServer(conf, newChiMux(handler))

	shutdownSignalChan := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go start(server)

	<-shutdownSignalChan

	logger.Log.Sugar().Info("received shutdown signal")
	shutdown(server)
}
