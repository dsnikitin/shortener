package main

import (
	"context"
	"net/http"
	"time"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/logger"
)

func initServer(conf *config.Config, router http.Handler) *http.Server {
	return &http.Server{
		Addr:         conf.ServerAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
}

func start(server *http.Server) {
	logger.Log.Sugar().Infow("starting server", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Log.Sugar().Fatalw("server starting failed", "error", err)
	}
}

func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Sugar().Errorw("server shutdown failed", "error", err)
	} else {
		logger.Log.Sugar().Infow("server shutdown completed")
	}
}
