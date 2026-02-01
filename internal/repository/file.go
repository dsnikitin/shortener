package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

const queueSize int = 1000

type File struct {
	mu            sync.RWMutex
	urlsCache     map[string]models.URL
	userUrlsCache map[uuid.UUID]map[string]struct{}
	file          *os.File
	queue         chan models.URL
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
		urlsCache:     make(map[string]models.URL),
		userUrlsCache: make(map[uuid.UUID]map[string]struct{}),
		file:          file,
		queue:         make(chan models.URL, queueSize),
		shutdown:      make(chan struct{}),
	}

	if err := r.loadToCache(); err != nil {
		if closeErr := r.file.Close(); closeErr != nil {
			logger.Log.Errorw("Failed to close file", "error", closeErr)
		}

		return nil, err
	}

	r.wg.Add(1)
	go r.asyncWriter()

	return r, nil
}

func (r *File) Save(ctx context.Context, url models.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.urlsCache[url.ID]; ok {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	r.urlsCache[url.ID] = url

	if _, ok := r.userUrlsCache[url.CreatorID]; ok {
		r.userUrlsCache[url.CreatorID][url.ID] = struct{}{}
	} else {
		r.userUrlsCache[url.CreatorID] = map[string]struct{}{url.ID: {}}
	}

	select {
	case r.queue <- url:
		return nil
	case <-r.shutdown:
		return errors.New("file storage closed")
	}
}

func (r *File) SaveMany(ctx context.Context, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(ctx, urls[i]); err != nil {
			return fmt.Errorf("save one: %w", err)
		}
	}

	return nil
}

func (r *File) GetURL(ctx context.Context, id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urlsCache[id]; ok {
		return url, nil
	}

	return models.URL{}, errx.ErrNotFound
}

func (r *File) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	var urls []models.URL

	ids, ok := r.userUrlsCache[userID]
	if !ok {
		return urls, nil
	}

	for id := range ids {
		urls = append(urls, r.urlsCache[id])
	}

	return urls, nil
}

func (r *File) DeleteURLs(ctx context.Context, deletableURLs []models.DeletableURL) {
	for i, deletableURL := range deletableURLs {
		select {
		case <-ctx.Done():
			logger.Log.Warnw("Context done", "not deleted urls", deletableURLs[i:])
			return
		default:
			if ids, ok := r.userUrlsCache[deletableURL.CreatorID]; ok {
				if _, ok := ids[deletableURL.ID]; ok {
					if url, ok := r.urlsCache[deletableURL.ID]; ok {
						url.IsDeleted = true
						r.urlsCache[deletableURL.ID] = url
						select {
						case r.queue <- url:
							continue
						case <-r.shutdown:
							logger.Log.Warnw("File storage closed", "not deleted urls", deletableURLs[i:])
						}
					}
				}
			}
		}
	}
}

func (r *File) PingDB(ctx context.Context) error {
	return errors.New("not a db storage")
}

func (r *File) Close() {
	close(r.shutdown)
	r.wg.Wait()

	if err := r.file.Close(); err != nil {
		logger.Log.Errorw("Close file failed", "error", err)
	}
}

func (r *File) loadToCache() error {
	scanner := bufio.NewScanner(r.file)

	var url models.URL
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &url); err != nil {
			return err
		}

		r.urlsCache[url.ID] = url
		if _, ok := r.userUrlsCache[url.CreatorID]; ok {
			r.userUrlsCache[url.CreatorID][url.ID] = struct{}{}
		} else {
			r.userUrlsCache[url.CreatorID] = map[string]struct{}{url.ID: {}}
		}
	}

	return scanner.Err()
}

func (r *File) saveToFile(url models.URL) error {
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
				logger.Log.Errorw("Failed to save to file", "userURL", userURL, "error", err)
			}
		case <-r.shutdown:
			for {
				select {
				case userURL := <-r.queue:
					if err := r.saveToFile(userURL); err != nil {
						logger.Log.Errorw("Failed to save to file", "userURL", userURL, "error", err)
					}
				default:
					return
				}
			}
		}
	}
}
