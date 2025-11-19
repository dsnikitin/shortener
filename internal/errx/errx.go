package errx

import (
	"errors"

	"github.com/dsnikitin/shortener/internal/models"
)

var ErrNotFound = errors.New("id not found")

type ErrAlreadyExists struct {
	CorrelationID string
	URL           models.URL
	Err           error
}

func NewAlreadyExistsError(url models.URL, err error) error {
	return &ErrAlreadyExists{
		URL: url,
		Err: err,
	}
}

func (e *ErrAlreadyExists) Error() string {
	return e.Err.Error()
}

func (e *ErrAlreadyExists) Unwrap() error {
	return e.Err
}
