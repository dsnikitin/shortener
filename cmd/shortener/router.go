package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/go-chi/chi/v5"
)

func newChiMux(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(handler.Logging)

	r.With(handler.BodyMaxBytesReader).Post("/", http.HandlerFunc(h.Shorten))
	r.Get("/{id}", http.HandlerFunc(h.Redirect))

	r.Route("/api", func(r chi.Router) {
		r.With(handler.BodyMaxBytesReader).Post("/shorten", http.HandlerFunc(h.ShortenFromJSON))
	})

	return r
}
