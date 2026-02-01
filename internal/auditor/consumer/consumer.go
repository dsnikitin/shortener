package consumer

import (
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

// Consumer представляет базового потребителя событий.
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
		return errors.New("events queue if full")
	}
}

// Stop останавливает потребителя, завершая все горутины и ожидая их завершения.
func (c *Consumer) Stop() {
	close(c.shutdown)
	c.eg.Wait()
	logger.Log.Infof("Consumer %s stopped", c.id)
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
						logger.Log.Errorw("Failed to process event while shuting down", "event", event, "error", err)
					}
				default:
					return nil
				}
			}
		}
	}
}
