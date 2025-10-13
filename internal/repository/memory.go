package repository

import (
	"errors"
	"sync"

	"github.com/dsnikitin/shortener/internal/model"
)

type id = string
type original = string

type MemoryRepository struct {
	mu      sync.RWMutex
	storage map[id]original
}

func NewMemory() *MemoryRepository {
	return &MemoryRepository{
		mu:      sync.RWMutex{},
		storage: make(map[id]original),
	}
}

func (r *MemoryRepository) Save(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.storage[url.ID] = url.Original
	return nil
}

func (r *MemoryRepository) Get(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.storage[id]; ok {
		return &model.URL{ID: id, Original: url}, nil
	}

	return nil, errors.New("id id not found")
}
