package handler

import (
	"io"
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/go-chi/chi/v5"
)

type Service interface {
	CreateID(url string) (string, error)
	GetOriginal(id string) (string, error)
}

type Handler struct {
	s Service
}

func New(s Service) *Handler {
	return &Handler{s: s}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(bytes) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	id, err := h.s.CreateID(string(bytes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	io.WriteString(w, config.ShortURLBase+"/"+id)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" || len(id) > config.IDMaxLength {
		http.Error(w, "incorrect id", http.StatusBadRequest)
		return
	}

	url, err := h.s.GetOriginal(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
