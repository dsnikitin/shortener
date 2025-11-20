package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/go-chi/chi/v5"
)

type Service interface {
	CreateID(url string) (string, error)
	CreateIDs(req map[string]string) (map[string]string, error)
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

	w.Header().Set("Content-Type", "text/plain")
	status := http.StatusCreated

	id, err := h.s.CreateID(string(bodyBytes))
	if err != nil {
		var aeErr *errx.ErrAlreadyExists
		if !errors.As(err, &aeErr) {
			logger.Log.Sugar().Errorw("failed to create id", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		status = http.StatusConflict
		id = aeErr.URL.ID
	}

	w.WriteHeader(status)
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

	w.Header().Set("Content-Type", "application/json")
	status := http.StatusCreated

	id, err := h.s.CreateID(req.URL)
	if err != nil {
		var aeErr *errx.ErrAlreadyExists
		if !errors.As(err, &aeErr) {
			logger.Log.Sugar().Errorw("failed to create id", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		status = http.StatusConflict
		id = aeErr.URL.ID
	}

	w.WriteHeader(status)

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
		http.Error(w, "empty request", http.StatusBadRequest)
		return
	}

	originalURLs := make(map[string]string)
	for i := range req {
		if _, ok := originalURLs[req[i].CorrelationID]; ok {
			http.Error(w, fmt.Sprintf("duplicate correlationID %s", req[i].CorrelationID), http.StatusBadRequest)
			return
		}

		originalURLs[req[i].CorrelationID] = req[i].OriginalURL
	}

	ids, err := h.s.CreateIDs(originalURLs)
	if err != nil {
		var aeErr *errx.ErrAlreadyExists
		if !errors.As(err, &aeErr) {
			logger.Log.Sugar().Errorw("failed to create ids", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		resp := []models.ShortenBatchResponse{{
			CorrelationID: aeErr.CorrelationID,
			ShortURL:      h.conf.ShortURLBaseAddr + "/" + aeErr.URL.ID,
		}}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Log.Sugar().Errorw("encoding shorten batch response", "error", err)
		}
		return
	}

	h.sendBatchResponse(w, req, ids)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" || len(id) > config.IDMaxLength {
		http.Error(w, "incorrect id", http.StatusBadRequest)
		return
	}

	url, err := h.s.GetOriginal(id)
	if err != nil {
		if !errors.Is(err, err) {
			logger.Log.Sugar().Errorw("failed to get original url", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusNotFound)
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

func (h *Handler) sendBatchResponse(
	w http.ResponseWriter, req []models.ShortenBatchRequest, ids map[string]string,
) {
	resp := make([]models.ShortenBatchResponse, 0, len(req))
	for i := range req {
		id, ok := ids[req[i].CorrelationID]
		if !ok {
			logger.Log.Sugar().Errorw("missing result", "correlationID", req[i].CorrelationID)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		resp = append(resp, models.ShortenBatchResponse{
			CorrelationID: req[i].CorrelationID,
			ShortURL:      h.conf.ShortURLBaseAddr + "/" + id,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding shorten batch response", "error", err)
		return
	}
}
