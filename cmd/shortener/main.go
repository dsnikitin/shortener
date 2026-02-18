package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/logger"
)

var (
	buildVersion string // -ldflags -X main.buildVersion=v1.0.0
	buildDate    string // -ldflags -X 'main.buildDate=$(Get-Date -Format 'dd/MM/yyy HH:mm:ss')' для powershell
	buildCommit  string // -ldflags -X main.buildCommit=$(git rev-parse HEAD)
)

func main() {
	logBuildInfo()

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

func logBuildInfo() {
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	if buildDate == "" {
		buildDate = "N/A"
	}
	if buildCommit == "" {
		buildCommit = "N/A"
	}

	log.Println("Build version: ", buildVersion)
	log.Println("Build date: ", buildDate)
	log.Println("Build commit: ", buildCommit)
}
