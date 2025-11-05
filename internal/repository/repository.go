package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

const queueSize int = 1000

type Repository struct {
	mu       sync.RWMutex
	memory   map[string]string
	file     *os.File
	queue    chan *models.URL
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func New(filePath string) (*Repository, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	r := &Repository{
		mu:       sync.RWMutex{},
		memory:   make(map[string]string),
		file:     file,
		queue:    make(chan *models.URL, queueSize),
		shutdown: make(chan struct{}),
	}

	if err := r.loadToMemory(); err != nil {
		if closeErr := r.file.Close(); closeErr != nil {
			logger.Log.Sugar().Errorw("failed to close file", "error", closeErr)
		}

		return nil, err
	}

	r.wg.Add(1)
	go r.asyncWriter()

	return r, nil
}

func (r *Repository) Save(url *models.URL) error {
	r.mu.Lock()
	r.memory[url.ID] = url.Original
	r.mu.Unlock()

	for {
		select {
		case r.queue <- url:
			return nil
		case <-r.shutdown:
			return errors.New("repository closed")
		}
	}
}

func (r *Repository) Get(id string) (*models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.memory[id]; ok {
		return &models.URL{ID: id, Original: url}, nil
	}

	return nil, errors.New("id not found")
}

func (r *Repository) Close() {
	close(r.shutdown)
	r.wg.Wait()

	if err := r.file.Close(); err != nil {
		logger.Log.Sugar().Errorw("failed to close file", "error", err)
	}
}

func (r *Repository) loadToMemory() error {
	scanner := bufio.NewScanner(r.file)

	var urlEntry models.URL
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &urlEntry); err != nil {
			return err
		}

		r.memory[urlEntry.ID] = urlEntry.Original
	}

	return scanner.Err()
}

func (r *Repository) saveToFile(url *models.URL) error {
	data, err := json.Marshal(url)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = r.file.Write(data)
	return err
}

func (r *Repository) asyncWriter() {
	defer r.wg.Done()

	for {
		select {
		case url := <-r.queue:
			if err := r.saveToFile(url); err != nil {
				logger.Log.Sugar().Errorw("failed to save to file", "url", url, "error", err)
			}
		case <-r.shutdown:
			for {
				select {
				case url := <-r.queue:
					if err := r.saveToFile(url); err != nil {
						logger.Log.Sugar().Errorw("failed to save to file", "url", url, "error", err)
					}
				default:
					return
				}
			}
		}
	}
}
