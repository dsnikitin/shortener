package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/logger"
)

// заполнится при линковке с флагом
var (
	buildVersion string = "N/A" // -ldflags "-X main.buildVersion=v1.0.0"
	buildDate    string = "N/A" // -ldflags "-X 'main.buildDate=$(Get-Date -Format 'dd/MM/yyy HH:mm:ss')'" для powershell
	buildCommit  string = "N/A" // -ldflags "-X main.buildCommit=$(git rev-parse HEAD)"
)

func main() {
	log.Println("Build version: ", buildVersion)
	log.Println("Build date: ", buildDate)
	log.Println("Build commit: ", buildCommit)

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to init config: %s", err)
	}

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		log.Fatalf("Failed to init logger: %s", err)
	}

	app := newApp(cfg)

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go app.startHTTPServer()
	go app.startGRPCServer()

	<-shutdownSignal

	logger.Log.Infow("Received shutdown signal")
	app.shutdown()
}
