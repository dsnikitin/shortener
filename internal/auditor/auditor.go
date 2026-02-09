package auditor

import (
	"golang.org/x/sync/errgroup"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

// Consumer определяет интерфейс потребителя событий.
type Consumer interface {
	GetID() string
	Consume(event models.Event) error
	Stop()
}

// Auditor представляет аудитора событий, который отправляет события зарегистрированным потребителям.
type Auditor struct {
	consumers map[string]Consumer

	eg          errgroup.Group
	inputEvents chan models.Event
	shutdown    chan struct{}
}

// New создает нового аудитора событий.
// eventsLimit - максимальный размер очереди входящих событий,
// consumers - список потребителей для регистрации.
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

// PublishEvent публикует событие для обработки зарегистрированными потребителями.
// Если очередь заполнена, то событие отбрасывается с предупреждением в лог.
func (a *Auditor) PublishEvent(event models.Event) {
	select {
	case a.inputEvents <- event:
	default:
		logger.Log.Warnw("Event was not added to queue because it was full", "event", event)
	}
}

// Stop останавливает аудитора и всех зарегистрированных потребителей.
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
