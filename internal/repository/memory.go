package repository

import (
	"errors"
	"sync"

	"github.com/dsnikitin/shortener/internal/models"
)

type Memory struct {
	mu      sync.RWMutex
	storage map[string]string
}

func NewMemory() *Memory {
	return &Memory{
		mu:      sync.RWMutex{},
		storage: make(map[string]string),
	}
}

func (r *Memory) Save(url *models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.storage[url.ID]; ok {
		return errors.New("id already exists")
	}

	r.storage[url.ID] = url.Original
	return nil
}

func (r *Memory) Get(id string) (*models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.storage[id]; ok {
		return &models.URL{ID: id, Original: url}, nil
	}

	return nil, errors.New("id not found")
}

func (r *Memory) PingDB() error {
	return errors.New("not a db storage")
}

func (r *Memory) Close() {}
