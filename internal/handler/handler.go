package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/go-chi/chi/v5"
)

type Service interface {
	CreateID(url string) (string, error)
	CreateIDs(req []models.ShortenBatchRequest) (map[string]string, error)
	GetOriginal(id string) (string, error)
	PingDB() error
}

type Handler struct {
	conf *config.Config
	s    Service
}

func New(conf *config.Config, s Service) *Handler {
	return &Handler{
		conf: conf,
		s:    s,
	}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Sugar().Errorw("cannot read request text body", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(bodyBytes) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	id, err := h.s.CreateID(string(bodyBytes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)

	io.WriteString(w, h.conf.ShortURLBaseAddr+"/"+id)
}

func (h *Handler) ShortenFromJSON(w http.ResponseWriter, r *http.Request) {
	var req models.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log.Sugar().Errorw("cannot read request JSON body", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(req.URL) == 0 {
		http.Error(w, "empty url", http.StatusBadRequest)
		return
	}

	id, err := h.s.CreateID(req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := models.ShortenResponse{Result: h.conf.ShortURLBaseAddr + "/" + id}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding shorten response", "error", err)
		return
	}
}

func (h *Handler) ShortenBatch(w http.ResponseWriter, r *http.Request) {
	var req []models.ShortenBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log.Sugar().Errorw("cannot read shorten batch request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(req) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	ids, err := h.s.CreateIDs(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := make([]models.ShortenBatchResponse, 0, len(req))
	for i := range req {
		id, ok := ids[req[i].CorrelationID]
		if !ok {
			http.Error(w, "result not exists", http.StatusInternalServerError)
			return
		}
		resp = append(resp, models.ShortenBatchResponse{
			CorrelationID: req[i].CorrelationID,
			ShortURL:      h.conf.ShortURLBaseAddr + "/" + id,
		})
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding shorten batch response", "error", err)
		return
	}
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

func (h *Handler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := h.s.PingDB(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}
