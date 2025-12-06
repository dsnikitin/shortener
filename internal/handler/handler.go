package handler

import (
	"context"
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
	"github.com/google/uuid"
)

type Service interface {
	CreateID(ctx context.Context, userID uuid.UUID, url string) (string, error)
	CreateIDs(ctx context.Context, userID uuid.UUID, req map[string]string) (map[string]string, error)
	GetURL(ctx context.Context, id string) (models.URL, error)
	PingDB(ctx context.Context) error
	GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error)
	DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error
}

type Handler struct {
	shortURLBaseAddr string
	s                Service
}

func New(shortURLBaseAddr string, s Service) *Handler {
	return &Handler{
		shortURLBaseAddr: shortURLBaseAddr,
		s:                s,
	}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

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

	id, err := h.s.CreateID(r.Context(), userID, string(bodyBytes))
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
	io.WriteString(w, h.shortURLBaseAddr+"/"+id)
}

func (h *Handler) ShortenFromJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

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

	id, err := h.s.CreateID(r.Context(), userID, req.URL)
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

	resp := models.ShortenResponse{Result: h.shortURLBaseAddr + "/" + id}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding shorten response", "error", err)
		return
	}
}

func (h *Handler) ShortenBatch(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

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

	ids, err := h.s.CreateIDs(r.Context(), userID, originalURLs)
	if err != nil {
		var aeErr *errx.ErrAlreadyExists
		if !errors.As(err, &aeErr) {
			logger.Log.Sugar().Errorw("failed to create ids", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		resp := []models.ShortenBatchResponse{{
			CorrelationID: aeErr.CorrelationID,
			ShortURL:      h.shortURLBaseAddr + "/" + aeErr.URL.ID,
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

	url, err := h.s.GetURL(r.Context(), id)
	if err != nil {
		if !errors.Is(err, errx.ErrNotFound) {
			logger.Log.Sugar().Errorw("failed to get original url", "error", err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	if url.IsDeleted {
		w.WriteHeader(http.StatusGone)
		return
	}

	http.Redirect(w, r, url.Original, http.StatusTemporaryRedirect)
}

func (h *Handler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := h.s.PingDB(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

	urls, err := h.s.GetUserURLs(r.Context(), userID)
	if err != nil {
		logger.Log.Sugar().Errorw("failed to get user urls", "userID", userID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]models.GetUserUrlsResponseItem, 0, len(urls))
	for i := range urls {
		if !urls[i].IsDeleted {
			resp = append(resp, models.GetUserUrlsResponseItem{
				ShortURL:    h.shortURLBaseAddr + "/" + urls[i].ID,
				OriginalURL: urls[i].Original,
			})
		}
	}

	if len(resp) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding user urls response", "error", err)
		return
	}
}

func (h *Handler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUserID(w, r)
	if !ok {
		return
	}

	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		logger.Log.Sugar().Errorw("cannot read delete user urls request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(ids) == 0 {
		http.Error(w, "empty request", http.StatusBadRequest)
		return
	}

	if err := h.s.DeleteUserURLs(r.Context(), userID, ids); err != nil {
		logger.Log.Sugar().Errorw("delete user urls", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
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
			ShortURL:      h.shortURLBaseAddr + "/" + id,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Log.Sugar().Errorw("encoding shorten batch response", "error", err)
		return
	}
}

func parseUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	strUserID := r.Header.Get("x-user-id")
	userID, err := uuid.Parse(strUserID)
	if err != nil {
		logger.Log.Sugar().Errorw("failed to parse userID to uuid", "userID", strUserID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return uuid.Nil, false
	}

	return userID, true
}
