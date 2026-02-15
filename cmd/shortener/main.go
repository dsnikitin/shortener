package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/logger"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("config init error: %s", err)
	}

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		log.Fatalf("logger init error: %s", err)
	}

	app := newApp(cfg)

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go app.start()

	<-shutdownSignal

	logger.Log.Infow("Received shutdown signal")
	app.shutdown()
}
