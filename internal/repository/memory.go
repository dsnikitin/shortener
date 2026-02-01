package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

type Memory struct {
	mu       sync.RWMutex
	urls     map[string]models.URL
	userURLs map[uuid.UUID]map[string]struct{}
}

func NewMemory() *Memory {
	return &Memory{
		mu:       sync.RWMutex{},
		urls:     make(map[string]models.URL),
		userURLs: make(map[uuid.UUID]map[string]struct{}),
	}
}

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

func (r *Memory) SaveMany(ctx context.Context, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(ctx, urls[i]); err != nil {
			return fmt.Errorf("save one: %w", err)
		}
	}

	return nil
}

func (r *Memory) GetURL(ctx context.Context, id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urls[id]; ok {
		return url, nil
	}

	return models.URL{}, errx.ErrNotFound
}

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

func (r *Memory) DeleteURLs(ctx context.Context, deletableURLs []models.DeletableURL) {
	for i, deletableURL := range deletableURLs {
		select {
		case <-ctx.Done():
			logger.Log.Warnw("Context done", "not deleted urls", deletableURLs[i:])
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

func (r *Memory) PingDB(ctx context.Context) error {
	return errors.New("not a db storage")
}

func (r *Memory) Close() {}
