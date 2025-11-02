package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/go-chi/chi/v5"
)

func newChiMux(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()

	r.With(logging, bodyMaxBytesReader).Post("/", http.HandlerFunc(h.Shorten))
	r.With(logging).Get("/{id}", http.HandlerFunc(h.Redirect))

	return r
}
