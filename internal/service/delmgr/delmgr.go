package delmgr

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
	DeleteUserURLs(ctx context.Context, data []models.DeletableURL)
}

type DelMgr struct {
	r Repository

	inputCh  chan models.DeletableURL
	outputCh chan []models.DeletableURL

	wg     sync.WaitGroup
	stopCh chan struct{}

	mu      sync.Mutex
	buffer  []models.DeletableURL
	flushCh chan struct{}
}

func New(r Repository) *DelMgr {
	m := &DelMgr{
		r:        r,
		inputCh:  make(chan models.DeletableURL),
		outputCh: make(chan []models.DeletableURL, 1),
		stopCh:   make(chan struct{}),
		buffer:   make([]models.DeletableURL, 0, batchSize),
		flushCh:  make(chan struct{}, 1),
	}

	return m
}

func (m *DelMgr) Run() {
	for range inputWorkers {
		m.wg.Add(1)
		go m.inputWorker()
	}

	m.wg.Add(1)
	go m.flusher()

	m.wg.Add(1)
	go m.writer()
}

func (m *DelMgr) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	logger.Log.Sugar().Info("deletion manager stopped")
}

func (m *DelMgr) DeleteURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
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

func (m *DelMgr) inputWorker() {
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

func (m *DelMgr) addToBuffer(data models.DeletableURL) {
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

func (m *DelMgr) flusher() {
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

func (m *DelMgr) flushBuffer() {
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

func (m *DelMgr) writer() {
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
			m.r.DeleteUserURLs(ctx, batch)
			cancel()
		}
	}
}
