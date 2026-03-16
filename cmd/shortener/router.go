package main

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/middleware"
)

func initChiRouter(cfg *config.Config, h *handler.HTTPHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Auth(cfg.JWTSigningKey))
	r.Use(middleware.Logging)
	r.Use(middleware.GzipCompress)

	r.With(middleware.BodyMaxBytesReader(config.OriginalURLMaxLength)).
		Post("/", http.HandlerFunc(h.Shorten))
	r.Get("/{id}", http.HandlerFunc(h.Redirect))
	r.Get("/ping", http.HandlerFunc(h.PingDB))

	r.Route("/api", func(r chi.Router) {
		r.With(middleware.SubnetGuard(cfg.TrustedSubnet)).
			Get("/internal/stats", http.HandlerFunc(h.GetStats))
		r.With(middleware.BodyMaxBytesReader(config.OriginalURLMaxLength)).
			Post("/shorten", http.HandlerFunc(h.ShortenFromJSON))
		r.With(middleware.BodyMaxBytesReader(config.OriginalURLMaxBatchLength)).
			Post("/shorten/batch", http.HandlerFunc(h.ShortenBatch))
		r.Get("/user/urls", http.HandlerFunc(h.GetUserURLs))
		r.With(middleware.BodyMaxBytesReader(config.OriginalURLMaxBatchLength)).
			Delete("/user/urls", http.HandlerFunc(h.DeleteUserURLs))
	})

	if cfg.IsDevelop {
		r.Mount("/debug", chimiddleware.Profiler())
	}

	return r
}
