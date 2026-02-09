package errx

import (
	"errors"

	"github.com/dsnikitin/shortener/internal/models"
)

// ErrNotFound ошибка, возникающая когда запрашиваемый ресурс не найден.
var ErrNotFound = errors.New("id not found")

// ErrAlreadyExists ошибка, возникающая при попытке создать дубликат ресурса.
type ErrAlreadyExists struct {
	CorrelationID string
	URL           models.URL
	Err           error
}

// NewAlreadyExistsError создает новую ошибку ErrAlreadyExists.
func NewAlreadyExistsError(url models.URL, err error) error {
	return &ErrAlreadyExists{
		URL: url,
		Err: err,
	}
}

// Error возвращает строковое представление ошибки.
func (e *ErrAlreadyExists) Error() string {
	return e.Err.Error()
}

// Unwrap возвращает вложенную ошибку.
func (e *ErrAlreadyExists) Unwrap() error {
	return e.Err
}
