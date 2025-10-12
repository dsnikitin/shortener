package handler

import (
	"io"
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
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
	if r.Method != http.MethodPost {
		http.Error(w, "only POST requests are allowed", http.StatusBadRequest)
		return
	}

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

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusCreated)

	io.WriteString(w, config.ShortURLBase+"/"+id)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "оnly GET requests are allowed", http.StatusBadRequest)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	url, err := h.s.GetOriginal(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
