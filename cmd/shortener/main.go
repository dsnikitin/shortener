package main

import (
	"log"
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
)

func main() {
	c := config.NewFromArgs()
	r := repository.NewMemory()
	s := service.New(r)
	h := handler.New(c, s)

	if err := logger.Initialize(c.LogLevel); err != nil {
		log.Fatalf("init logger error: %s", err)
	}

	logger.Log.Sugar().Infow("Running server", "address", c.ServerAddr)

	if err := http.ListenAndServe(c.ServerAddr, newChiMux(h)); err != nil {
		logger.Log.Sugar().Fatalw("running server", "error", err)
	}
}
