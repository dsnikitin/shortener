package consumer

import (
	"encoding/json"
	"os"

	"github.com/dsnikitin/shortener/internal/models"
	"github.com/pkg/errors"
)

type File struct {
	Consumer
	file *os.File
}

func NewFile(id, path string, eventsLimit int) (*File, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, errors.Wrap(err, "open file")
	}

	c := &File{
		Consumer: Consumer{
			id:       id,
			events:   make(chan models.Event, eventsLimit),
			shutdown: make(chan struct{}),
		},
		file: file,
	}

	c.eg.Go(func() error {
		return c.process(c.consume)
	})

	return c, nil
}

func (c *File) consume(event models.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "marshal event")
	}

	data = append(data, '\n')

	_, err = c.file.Write(data)
	return errors.Wrap(err, "write event to file")
}
