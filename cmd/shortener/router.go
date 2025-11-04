package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func newChiMux(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logging)
	r.Use(middleware.GzipCompress)

	r.With(middleware.BodyMaxBytesReader).Post("/", http.HandlerFunc(h.Shorten))
	r.Get("/{id}", http.HandlerFunc(h.Redirect))

	r.Route("/api", func(r chi.Router) {
		r.With(middleware.BodyMaxBytesReader).Post("/shorten", http.HandlerFunc(h.ShortenFromJSON))
	})

	return r
}
