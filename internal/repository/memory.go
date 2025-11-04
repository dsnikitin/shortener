package repository

import (
	"errors"
	"sync"

	"github.com/dsnikitin/shortener/internal/models"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	storage map[string]string
}

func NewMemory() *MemoryRepository {
	return &MemoryRepository{
		mu:      sync.RWMutex{},
		storage: make(map[string]string),
	}
}

func (r *MemoryRepository) Save(url *models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.storage[url.ID] = url.Original
	return nil
}

func (r *MemoryRepository) Get(id string) (*models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.storage[id]; ok {
		return &models.URL{ID: id, Original: url}, nil
	}

	return nil, errors.New("id not found")
}
