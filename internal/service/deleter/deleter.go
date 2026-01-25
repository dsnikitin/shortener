package deleter

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/google/uuid"
)

const dbRequestSecondsTimeout = 3
const flushSecondsInterval = 5
const batchSize = 100
const inputWorkers = 5

type Repository interface {
	DeleteURLs(ctx context.Context, data []models.DeletableURL)
}

type Deleter struct {
	r Repository

	inputCh  chan models.DeletableURL
	outputCh chan []models.DeletableURL

	wg     sync.WaitGroup
	stopCh chan struct{}

	mu      sync.Mutex
	buffer  []models.DeletableURL
	flushCh chan struct{}
}

func New(r Repository) *Deleter {
	m := &Deleter{
		r:        r,
		inputCh:  make(chan models.DeletableURL),
		outputCh: make(chan []models.DeletableURL, 1),
		stopCh:   make(chan struct{}),
		buffer:   make([]models.DeletableURL, 0, batchSize),
		flushCh:  make(chan struct{}, 1),
	}

	return m
}

func (m *Deleter) Run() {
	for range inputWorkers {
		m.wg.Add(1)
		go m.inputWorker()
	}

	m.wg.Add(1)
	go m.flusher()

	m.wg.Add(1)
	go m.writer()
}

func (m *Deleter) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	logger.Log.Sugar().Info("deletion manager stopped")
}

func (m *Deleter) DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.stopCh:
			return errors.New("deletion manager stopped")
		case m.inputCh <- models.DeletableURL{ID: id, CreatorID: userID}:
		}
	}

	return nil
}

func (m *Deleter) inputWorker() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case data, ok := <-m.inputCh:
			if !ok {
				return
			}

			m.addToBuffer(data)
		}
	}
}

func (m *Deleter) addToBuffer(data models.DeletableURL) {
	m.mu.Lock()
	m.buffer = append(m.buffer, data)
	if len(m.buffer) >= batchSize {
		select {
		case m.flushCh <- struct{}{}:
		default:
		}
	}
	m.mu.Unlock()
}

func (m *Deleter) flusher() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Second * flushSecondsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			m.flushBuffer()
			return
		case <-ticker.C:
			m.flushBuffer()
		case <-m.flushCh:
			m.flushBuffer()
		}
	}
}

func (m *Deleter) flushBuffer() {
	m.mu.Lock()
	if len(m.buffer) == 0 {
		m.mu.Unlock()
		return
	}

	batch := m.buffer
	m.buffer = make([]models.DeletableURL, 0, batchSize)
	m.mu.Unlock()

	m.outputCh <- batch
}

func (m *Deleter) writer() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case batch, ok := <-m.outputCh:
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*dbRequestSecondsTimeout)
			m.r.DeleteURLs(ctx, batch)
			cancel()
		}
	}
}
