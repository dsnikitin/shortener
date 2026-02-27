package deleter

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

const dbRequestSecondsTimeout = 5
const flushSecondsInterval = 3
const batchSize = 100
const inputWorkers = 5

// Repository определяет интерфейс репозитория для удаления URL.
type Repository interface {
	DeleteURLs(ctx context.Context, data []models.DeletableURL)
}

// Deleter представляет менеджер для асинхронного удаления URL.
// generate:reset
type Deleter struct {
	r Repository

	inputCh  chan models.DeletableURL
	outputCh chan []models.DeletableURL

	stopInputWrsCh chan struct{}
	stopFlusherCh  chan struct{}
	stopWriterCh   chan struct{}

	inputWrsWg sync.WaitGroup
	flusherWg  sync.WaitGroup
	writerWg   sync.WaitGroup

	mu      sync.Mutex
	buffer  []models.DeletableURL
	flushCh chan struct{}
}

// New создает новый менеджер удаления URL.
func New(r Repository) *Deleter {
	m := &Deleter{
		r:              r,
		inputCh:        make(chan models.DeletableURL),
		outputCh:       make(chan []models.DeletableURL, 1),
		stopInputWrsCh: make(chan struct{}),
		stopFlusherCh:  make(chan struct{}),
		stopWriterCh:   make(chan struct{}),
		buffer:         make([]models.DeletableURL, 0, batchSize),
		flushCh:        make(chan struct{}, 1),
	}

	m.run()
	return m
}

// Run запускает менеджер удаления URL.
func (d *Deleter) run() {
	for range inputWorkers {
		d.inputWrsWg.Add(1)
		go d.inputWorker()
	}

	d.flusherWg.Add(1)
	go d.flusher()

	d.writerWg.Add(1)
	go d.writer()
}

// Stop останавливает менеджер удаления URL.
func (d *Deleter) Stop(ctx context.Context) error {
	close(d.stopInputWrsCh)

	components := []struct {
		name       string
		wg         *sync.WaitGroup
		nextStopCh chan (struct{})
	}{
		{
			name:       "inputWorkers",
			wg:         &d.inputWrsWg,
			nextStopCh: d.stopFlusherCh,
		},
		{
			name:       "flusher",
			wg:         &d.flusherWg,
			nextStopCh: d.stopWriterCh,
		},
		{
			name: "writer",
			wg:   &d.writerWg,
		},
	}

	for _, c := range components {
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			select {
			case <-done: // всё же успели завершиться
			default:
				return errors.Wrapf(ctx.Err(), "stop component %s", c.name)
			}
		}

		if c.nextStopCh != nil {
			close(c.nextStopCh)
		}
	}

	return nil
}

// DeleteUserURLs добавляет URLs для удаления.
func (d *Deleter) DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.stopInputWrsCh:
			return errors.New("deletion manager stopped")
		case d.inputCh <- models.DeletableURL{ID: id, CreatorID: userID}:
		}
	}

	return nil
}

func (d *Deleter) inputWorker() {
	defer d.inputWrsWg.Done()

	for {
		select {
		case url := <-d.inputCh:
			d.addToBuffer(url)
		case <-d.stopInputWrsCh:
			for {
				select {
				case url := <-d.inputCh:
					d.addToBuffer(url)
				default:
					return
				}
			}
		}
	}
}

func (d *Deleter) addToBuffer(data models.DeletableURL) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.buffer = append(d.buffer, data)
	if len(d.buffer) >= batchSize {
		select {
		case d.flushCh <- struct{}{}:
		default:
		}
	}
}

func (d *Deleter) flusher() {
	defer d.flusherWg.Done()

	ticker := time.NewTicker(time.Second * flushSecondsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.flushBuffer()
		case <-d.flushCh:
			d.flushBuffer()
		case <-d.stopFlusherCh:
			d.flushBuffer()
			return
		}
	}
}

func (d *Deleter) flushBuffer() {
	d.mu.Lock()
	if len(d.buffer) == 0 {
		d.mu.Unlock()
		return
	}

	batch := d.buffer
	d.buffer = make([]models.DeletableURL, 0, batchSize)
	d.mu.Unlock()

	d.outputCh <- batch
}

func (d *Deleter) writer() {
	defer d.writerWg.Done()

	for {
		select {
		case urls := <-d.outputCh:
			d.processBatch(urls)
		case <-d.stopWriterCh:
			for {
				select {
				case urls := <-d.outputCh:
					d.processBatch(urls)
				default:
					return
				}
			}
		}
	}
}

func (d *Deleter) processBatch(urls []models.DeletableURL) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*dbRequestSecondsTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		d.r.DeleteURLs(ctx, urls)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		logger.Log.Warnw("Failed to delete urls due to context timeout", "urls", urls)
	}
}
