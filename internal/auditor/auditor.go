package auditor

import (
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
	"golang.org/x/sync/errgroup"
)

type Consumer interface {
	GetID() string
	Consume(event models.Event) error
	Stop()
}

type Auditor struct {
	consumers map[string]Consumer

	eg          errgroup.Group
	inputEvents chan models.Event
	shutdown    chan struct{}
}

func New(eventsLimit int, consumers ...Consumer) *Auditor {
	a := &Auditor{
		inputEvents: make(chan models.Event, eventsLimit),
		consumers:   make(map[string]Consumer, len(consumers)),
		shutdown:    make(chan struct{}),
	}

	for _, c := range consumers {
		a.consumers[c.GetID()] = c
	}

	a.eg.Go(a.process)

	return a
}

func (a *Auditor) PublishEvent(event models.Event) {
	select {
	case a.inputEvents <- event:
	default:
		logger.Log.Warnw("Event was not added to queue because it was full", "event", event)
	}
}

func (a *Auditor) Stop() {
	close(a.shutdown)
	for _, c := range a.consumers {
		c.Stop()
	}

	a.eg.Wait()
	logger.Log.Info("Auditor stopped")
}

func (a *Auditor) process() error {
	for {
		select {
		case event := <-a.inputEvents:
			a.publish(event)
		case <-a.shutdown:
			for {
				select {
				case event := <-a.inputEvents:
					a.publish(event)
				default:
					return nil
				}
			}
		}
	}
}

func (a *Auditor) publish(event models.Event) {
	for _, c := range a.consumers {
		if err := c.Consume(event); err != nil {
			logger.Log.Warnw("Event was not consumed",
				"consumer", c.GetID(), "event", event, "error", err,
			)
		}
	}
}
