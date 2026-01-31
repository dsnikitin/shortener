package consumer

import (
	"errors"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"golang.org/x/sync/errgroup"
)

type Consumer struct {
	id       string
	events   chan models.Event
	eg       errgroup.Group
	shutdown chan struct{}
}

func (c *Consumer) GetID() string {
	return c.id
}

func (c *Consumer) Consume(event models.Event) error {
	select {
	case c.events <- event:
		return nil
	default:
		return errors.New("events queue if full")
	}
}

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
