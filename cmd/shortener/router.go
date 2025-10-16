package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/go-chi/chi/v5"
)

func newChiMux(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()

	r.With(bodyMaxBytesReader()).Post("/", http.HandlerFunc(h.Shorten))
	r.Get("/{id}", http.HandlerFunc(h.Redirect))

	return r
}

func bodyMaxBytesReader() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, config.OriginalURLMaxLength)

			next.ServeHTTP(w, r)
		})
	}
}
