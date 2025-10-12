package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/handler"
)

func newServeMux(h *handler.Handler) *http.ServeMux {
	sm := http.NewServeMux()
	sm.Handle("/", http.HandlerFunc(h.Shorten))
	sm.Handle("/{id}", http.HandlerFunc(h.Redirect))

	return sm
}
