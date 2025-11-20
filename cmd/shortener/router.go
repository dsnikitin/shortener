package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func newChiMux(h *handler.Handler) *chi.Mux {
	maxBodySize := config.OriginalURLMaxLength
	maxBatchBodySize := config.OriginalURLMaxLength * config.MaxOriginalURLCountInBatch

	r := chi.NewRouter()
	r.Use(middleware.Logging)
	r.Use(middleware.GzipCompress)

	r.With(middleware.BodyMaxBytesReader(maxBodySize)).Post("/", http.HandlerFunc(h.Shorten))
	r.Get("/{id}", http.HandlerFunc(h.Redirect))
	r.Get("/ping", http.HandlerFunc(h.PingDB))

	r.Route("/api", func(r chi.Router) {
		r.With(middleware.BodyMaxBytesReader(maxBodySize)).Post("/shorten", http.HandlerFunc(h.ShortenFromJSON))
		r.With(middleware.BodyMaxBytesReader(maxBatchBodySize)).Post("/shorten/batch", http.HandlerFunc(h.ShortenBatch))
	})

	return r
}
