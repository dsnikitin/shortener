package consumer

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

// Consumer представляет базового потребителя событий.
// generate:reset
type Consumer struct {
	id       string
	events   chan models.Event
	eg       errgroup.Group
	shutdown chan struct{}
}

// GetID возвращает идентификатор потребителя.
func (c *Consumer) GetID() string {
	return c.id
}

// Consume добавляет событие в очередь потребителя.
// Возвращает ошибку, если очередь заполнена.
func (c *Consumer) Consume(event models.Event) error {
	select {
	case c.events <- event:
		return nil
	default:
		return errors.New("events queue is full")
	}
}

// Stop останавливает потребителя, завершая все горутины и ожидая их завершения.
func (c *Consumer) Stop(ctx context.Context) error {
	close(c.shutdown)

	done := make(chan struct{})
	go func() {
		if err := c.eg.Wait(); err != nil {
			logger.Log.Errorw(fmt.Sprintf("Error while waiting for consumer %s shutdown", c.id), "error", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		select {
		case <-done:
		default:
			return errors.Wrapf(ctx.Err(), "stop consumer %s", c.id)
		}
	}

	return nil
}

func (c *Consumer) process(fn func(event models.Event) error) error {
	for {
		select {
		case event := <-c.events:
			if err := fn(event); err != nil {
				logger.Log.Errorw("Failed to process event", "event", event, "error", err)
			}
		case <-c.shutdown:
			for {
				select {
				case event := <-c.events:
					if err := fn(event); err != nil {
						logger.Log.Errorw("Failed to process event while shutting down", "event", event, "error", err)
					}
				default:
					return nil
				}
			}
		}
	}
}
