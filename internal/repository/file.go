package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/google/uuid"
)

const queueSize int = 1000

type File struct {
	mu            sync.RWMutex
	urlsCache     map[string]string
	userUrlsCache map[uuid.UUID]map[string]struct{}
	file          *os.File
	queue         chan models.UserURL
	shutdown      chan struct{}
	wg            sync.WaitGroup
}

func NewFile(filePath string) (*File, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	r := &File{
		mu:            sync.RWMutex{},
		urlsCache:     make(map[string]string),
		userUrlsCache: make(map[uuid.UUID]map[string]struct{}),
		file:          file,
		queue:         make(chan models.UserURL, queueSize),
		shutdown:      make(chan struct{}),
	}

	if err := r.loadToCache(); err != nil {
		if closeErr := r.file.Close(); closeErr != nil {
			logger.Log.Sugar().Errorw("failed to close file", "error", closeErr)
		}

		return nil, err
	}

	r.wg.Add(1)
	go r.asyncWriter()

	return r, nil
}

func (r *File) Save(userID uuid.UUID, url models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.urlsCache[url.ID]; ok {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	r.urlsCache[url.ID] = url.Original

	if _, ok := r.userUrlsCache[userID]; ok {
		r.userUrlsCache[userID][url.ID] = struct{}{}
	} else {
		r.userUrlsCache[userID] = map[string]struct{}{url.ID: {}}
	}

	for {
		select {
		case r.queue <- models.UserURL{URL: url, CreatorID: userID}:
			return nil
		case <-r.shutdown:
			return errors.New("file storage closed")
		}
	}
}

func (r *File) SaveMany(userID uuid.UUID, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(userID, urls[i]); err != nil {
			return fmt.Errorf("save one: %w", err)
		}
	}

	return nil
}

func (r *File) GetURL(id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urlsCache[id]; ok {
		return models.URL{ID: id, Original: url}, nil
	}

	return models.URL{}, errx.ErrNotFound
}

func (r *File) GetUserURLs(userID uuid.UUID) ([]models.URL, error) {
	var urls []models.URL

	ids, ok := r.userUrlsCache[userID]
	if !ok {
		return urls, nil
	}

	for id := range ids {
		urls = append(urls, models.URL{
			ID:       id,
			Original: r.urlsCache[id],
		})
	}

	return urls, nil
}

func (r *File) PingDB() error {
	return errors.New("not a db storage")
}

func (r *File) Close() error {
	close(r.shutdown)
	r.wg.Wait()

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}

	return nil
}

func (r *File) loadToCache() error {
	scanner := bufio.NewScanner(r.file)

	var entry models.UserURL
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}

		r.urlsCache[entry.ID] = entry.Original
		if _, ok := r.userUrlsCache[entry.CreatorID]; ok {
			r.userUrlsCache[entry.CreatorID][entry.ID] = struct{}{}
		} else {
			r.userUrlsCache[entry.CreatorID] = map[string]struct{}{entry.ID: {}}
		}
	}

	return scanner.Err()
}

func (r *File) saveToFile(url models.UserURL) error {
	data, err := json.Marshal(url)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	_, err = r.file.Write(data)
	return err
}

func (r *File) asyncWriter() {
	defer r.wg.Done()

	for {
		select {
		case userURL := <-r.queue:
			if err := r.saveToFile(userURL); err != nil {
				logger.Log.Sugar().Errorw("failed to save to file", "userURL", userURL, "error", err)
			}
		case <-r.shutdown:
			for {
				select {
				case userURL := <-r.queue:
					if err := r.saveToFile(userURL); err != nil {
						logger.Log.Sugar().Errorw("failed to save to file", "userURL", userURL, "error", err)
					}
				default:
					return
				}
			}
		}
	}
}
