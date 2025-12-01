package repository

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/google/uuid"
)

type Memory struct {
	mu       sync.RWMutex
	urls     map[string]string
	userURLs map[uuid.UUID]map[string]struct{}
}

func NewMemory() *Memory {
	return &Memory{
		mu:       sync.RWMutex{},
		urls:     make(map[string]string),
		userURLs: make(map[uuid.UUID]map[string]struct{}),
	}
}

func (r *Memory) Save(userID uuid.UUID, url models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.urls[url.ID]; ok {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	r.urls[url.ID] = url.Original

	if _, ok := r.userURLs[userID]; ok {
		r.userURLs[userID][url.ID] = struct{}{}
	} else {
		r.userURLs[userID] = map[string]struct{}{url.ID: {}}
	}

	return nil
}

func (r *Memory) SaveMany(userID uuid.UUID, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(userID, urls[i]); err != nil {
			return fmt.Errorf("save one: %w", err)
		}
	}

	return nil
}

func (r *Memory) GetURL(id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urls[id]; ok {
		return models.URL{ID: id, Original: url}, nil
	}

	return models.URL{}, errx.ErrNotFound
}

func (r *Memory) GetUserURLs(userID uuid.UUID) ([]models.URL, error) {
	var urls []models.URL

	ids, ok := r.userURLs[userID]
	if !ok {
		return urls, nil
	}

	for id := range ids {
		urls = append(urls, models.URL{
			ID:       id,
			Original: r.urls[id],
		})
	}

	return urls, nil
}

func (r *Memory) PingDB() error {
	return errors.New("not a db storage")
}

func (r *Memory) Close() error {
	return nil
}
