package repository

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

// Memory представляет хранилище URL в памяти.
// generate:reset
type Memory struct {
	mu       sync.RWMutex
	urls     map[string]models.URL
	userURLs map[uuid.UUID]map[string]struct{}
}

// NewMemory создает новое хранилище в памяти.
func NewMemory() *Memory {
	return &Memory{
		mu:       sync.RWMutex{},
		urls:     make(map[string]models.URL),
		userURLs: make(map[uuid.UUID]map[string]struct{}),
	}
}

// Save сохраняет URL в памяти.
func (r *Memory) Save(ctx context.Context, url models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.urls[url.ID]; ok {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	r.urls[url.ID] = url

	if _, ok := r.userURLs[url.CreatorID]; ok {
		r.userURLs[url.CreatorID][url.ID] = struct{}{}
	} else {
		r.userURLs[url.CreatorID] = map[string]struct{}{url.ID: {}}
	}

	return nil
}

// SaveMany сохраняет несколько URLs в памяти.
func (r *Memory) SaveMany(ctx context.Context, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(ctx, urls[i]); err != nil {
			logger.Log.Warnw("Failed to save urls", "not saved urls", urls[i:])
			return errors.Wrap(err, "save one")
		}
	}

	return nil
}

// GetURL возвращает URL по его короткой ссылке.
func (r *Memory) GetURL(ctx context.Context, id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urls[id]; ok {
		return url, nil
	}

	return models.URL{}, errx.ErrNotFound
}

// GetUserURLs возвращает все URLs пользователя.
func (r *Memory) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	var res []models.URL

	ids, ok := r.userURLs[userID]
	if !ok {
		return res, nil
	}

	for id := range ids {
		res = append(res, r.urls[id])
	}

	return res, nil
}

// DeleteURLs помечает URLs как удаленные.
func (r *Memory) DeleteURLs(ctx context.Context, deletableURLs []models.DeletableURL) {
	for i, deletableURL := range deletableURLs {
		select {
		case <-ctx.Done():
			logger.Log.Warnw("Failed to delete urls because request context done", "not deleted urls", deletableURLs[i:])
			return
		default:
			if ids, ok := r.userURLs[deletableURL.CreatorID]; ok {
				if _, ok := ids[deletableURL.ID]; ok {
					if url, ok := r.urls[deletableURL.ID]; ok {
						url.IsDeleted = true
						r.urls[deletableURL.ID] = url
					}
				}
			}
		}
	}
}

// PingDB проверяет соединение с хранилищем (не реализовано для хранилища в памяти).
func (r *Memory) PingDB(ctx context.Context) error {
	return errors.New("not a db storage")
}

// Close закрывает хранилище в памяти.
func (r *Memory) Close(context.Context) error {
	return nil
}
