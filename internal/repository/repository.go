package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/dsnikitin/shortener/internal/models"
)

type Repository struct {
	mu     sync.RWMutex
	memory map[string]string
	file   *os.File
}

func New(fileStorage *os.File) (*Repository, error) {
	if fileStorage == nil {
		return nil, errors.New("file storage is nil")
	}

	r := &Repository{
		mu:     sync.RWMutex{},
		memory: make(map[string]string),
		file:   fileStorage,
	}

	return r, r.loadToMemory()
}

func (r *Repository) Save(url *models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.memory[url.ID] = url.Original
	err := r.saveEntryToFile(url)

	return err
}

func (r *Repository) Get(id string) (*models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.memory[id]; ok {
		return &models.URL{ID: id, Original: url}, nil
	}

	return nil, errors.New("id not found")
}

func (r *Repository) loadToMemory() error {
	scanner := bufio.NewScanner(r.file)

	for scanner.Scan() {
		urlEntry := models.URL{}
		if err := json.Unmarshal(scanner.Bytes(), &urlEntry); err != nil {
			return err
		}

		r.memory[urlEntry.ID] = urlEntry.Original
	}

	return scanner.Err()
}

func (r *Repository) saveEntryToFile(url *models.URL) error {
	data, err := json.Marshal(url)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = r.file.Write(data)
	return err
}
