package consumer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/models"
)

// Remote представляет удаленного потребителя событий, который отправляет события по HTTP.
type Remote struct {
	Consumer
	url    string
	client *http.Client
}

// NewRemote создает нового удаленного потребителя событий.
// id - идентификатор потребителя,
// url - URL удаленного сервера,
// eventsLimit - максимальный размер очереди событий.
func NewRemote(id, url string, eventsLimit int) (*Remote, error) {
	c := &Remote{
		Consumer: Consumer{
			id:       id,
			events:   make(chan models.Event, eventsLimit),
			shutdown: make(chan struct{}),
		},
		url:    url,
		client: newClient(),
	}

	c.eg.Go(func() error {
		return c.process(c.consume)
	})

	return c, nil
}

// HealthCheck выполняет проверку доступности удаленного сервера.
func (c *Remote) HealthCheck() error {
	if err := c.sendRequest(http.MethodHead, bytes.NewReader(nil)); err != nil {
		return errors.Wrap(err, "send request")
	}

	return nil
}

func (c *Remote) consume(event models.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "marshal event")
	}

	if err := c.sendRequest(http.MethodPost, bytes.NewReader(data)); err != nil {
		return errors.Wrap(err, "send event to remote consumer")
	}

	return nil
}

func (c *Remote) sendRequest(method string, body *bytes.Reader) error {
	req, err := http.NewRequest(method, c.url, body)
	if err != nil {
		return errors.Wrap(err, "create health check request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "health check request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.Errorf("server returned error status: %d", resp.StatusCode)
	}

	return nil
}

func newClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}
