package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

const queueSize int = 1000

// File представляет файловое хранилище URL.
// generate:reset
type File struct {
	mu            sync.RWMutex
	urlsCache     map[string]models.URL
	userUrlsCache map[uuid.UUID]map[string]struct{}
	file          *os.File
	queue         chan models.URL
	shutdown      chan struct{}
	wg            sync.WaitGroup
}

// NewFile создает новое файловое хранилище.
func NewFile(filePath string) (*File, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600) // #nosec G304
	if err != nil {
		return nil, errors.Wrap(err, "open file")
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

		return nil, errors.Wrap(err, "load to cache")
	}

	r.wg.Add(1)
	go r.writer()

	return r, nil
}

// Save сохраняет URL в файловом хранилище.
func (r *File) Save(ctx context.Context, url models.URL) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.shutdown:
		return errors.New("file repository closed")
	default:
		r.mu.Lock()
		if _, ok := r.urlsCache[url.ID]; ok {
			r.mu.Unlock()
			return errx.NewAlreadyExistsError(url, errors.New("already exists"))
		}

		// обновляем кэш
		r.urlsCache[url.ID] = url
		if _, ok := r.userUrlsCache[url.CreatorID]; ok {
			r.userUrlsCache[url.CreatorID][url.ID] = struct{}{}
		} else {
			r.userUrlsCache[url.CreatorID] = map[string]struct{}{url.ID: {}}
		}
		r.mu.Unlock()

		// отправляем на запись в файл
		r.queue <- url
		return nil
	}
}

// SaveMany сохраняет несколько URLs в файловом хранилище.
func (r *File) SaveMany(ctx context.Context, urls []models.URL) error {
	for i := range urls {
		if err := r.Save(ctx, urls[i]); err != nil {
			logger.Log.Warnw("Failed to save urls", "not saved urls", urls[i:])
			return errors.Wrap(err, "save one")
		}
	}

	return nil
}

// GetURL возвращает URL по его короткой ссылке.
func (r *File) GetURL(ctx context.Context, id string) (models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if url, ok := r.urlsCache[id]; ok {
		return url, nil
	}

	return models.URL{}, errx.ErrNotFound
}

// GetUserURLs возвращает все URLs пользователя.
func (r *File) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

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

// DeleteURLs помечает URLs как удаленные.
func (r *File) DeleteURLs(ctx context.Context, deletableURLs []models.DeletableURL) {
	for i, deletableURL := range deletableURLs {
		select {
		case <-ctx.Done():
			logger.Log.Warnw("Failed to delete urls because request context done", "not deleted urls", deletableURLs[i:])
			return
		case <-r.shutdown:
			logger.Log.Warnw("Failed to delete urls because file repository closed", "not deleted urls", deletableURLs[i:])
			return
		default:
			r.mu.Lock()
			if ids, ok := r.userUrlsCache[deletableURL.CreatorID]; ok {
				if _, ok := ids[deletableURL.ID]; ok {
					if url, ok := r.urlsCache[deletableURL.ID]; ok {
						url.IsDeleted = true
						r.urlsCache[deletableURL.ID] = url
						r.mu.Unlock()

						r.queue <- url
						continue
					}
				}
			}
			r.mu.Unlock()
		}
	}
}

func (r *File) GetStats(ctx context.Context) (models.Stats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return models.Stats{
		URLs:  len(r.urlsCache),
		Users: len(r.userUrlsCache),
	}, nil
}

// PingDB проверяет соединение с хранилищем (не реализовано для файлового хранилища).
func (r *File) PingDB(ctx context.Context) error {
	return errors.New("not a db storage")
}

// Close закрывает файловое хранилище.
func (r *File) Close(ctx context.Context) error {
	defer func() {
		if err := r.file.Close(); err != nil {
			logger.Log.Errorw("Failed to close file", "error", err)
		}
	}()

	close(r.shutdown)
	done := make(chan struct{})

	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		select {
		case <-done:
		default:
			return errors.Wrap(ctx.Err(), "close file repository")
		}
	}

	return nil
}

func (r *File) loadToCache() error {
	scanner := bufio.NewScanner(r.file)

	var url models.URL
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &url); err != nil {
			return errors.Wrap(err, "unmarshal url from file")
		}

		r.urlsCache[url.ID] = url
		if _, ok := r.userUrlsCache[url.CreatorID]; ok {
			r.userUrlsCache[url.CreatorID][url.ID] = struct{}{}
		} else {
			r.userUrlsCache[url.CreatorID] = map[string]struct{}{url.ID: {}}
		}
	}

	return errors.Wrap(scanner.Err(), "scanner error")
}

func (r *File) writer() {
	defer r.wg.Done()

	for {
		select {
		case url := <-r.queue:
			if err := r.saveToFile(url); err != nil {
				logger.Log.Errorw("Failed to save url to file", "url", url, "error", err)
			}
		case <-r.shutdown:
			for {
				select {
				case url := <-r.queue:
					if err := r.saveToFile(url); err != nil {
						logger.Log.Errorw("Failed to save url to file while shutting down", "url", url, "error", err)
					}
				default:
					return
				}
			}
		}
	}
}

func (r *File) saveToFile(url models.URL) error {
	data, err := json.Marshal(url)
	if err != nil {
		return errors.Wrap(err, "marshal url")
	}

	data = append(data, '\n')

	_, err = r.file.Write(data)
	return errors.Wrap(err, "write url to file")
}
