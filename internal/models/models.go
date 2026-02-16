package models

import "github.com/google/uuid"

// URL представляет модель ссылки.
// ID - сокращенная ссылка без схемы,
// Original - оригинальная ссылка,
// CreatorID - ID пользователя-создателя,
// IsDeleted - флаг о том, что ссылка удалена.
// generate:reset
type URL struct {
	ID        string
	Original  string
	CreatorID uuid.UUID
	IsDeleted bool
}

// DeletableURL представляет модель ссылки для удаления.
// generate:reset
type DeletableURL struct {
	ID        string
	CreatorID uuid.UUID
}

// ShortenRequest представляет запрос на создание короткой ссылки.
// generate:reset
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse представляет ответ на создание короткой ссылки.
// generate:reset
type ShortenResponse struct {
	Result string `json:"result"`
}

// ShortenBatchRequest представляет запрос на создание нескольких коротких ссылок.
// generate:reset
type ShortenBatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// ShortenBatchResponse представляет ответ на создание нескольких коротких ссылок.
// generate:reset
type ShortenBatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// GetUserUrlsResponseItem представляет элемент ответа со списком ссылок пользователя.
// generate:reset
type GetUserUrlsResponseItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Action представляет тип совершенного действия.
type Action string

const (
	// Shorten действие создания короткой ссылки.
	Shorten Action = "shorten"
	// Follow действие перехода по короткой ссылке.
	Follow Action = "follow"
)

// Event представляет событие аудита.
// generate:reset
type Event struct {
	Timestamp   int64  `json:"ts"`
	Action      Action `json:"action"`
	UserID      string `json:"user_id,omitempty"`
	OriginalURL string `json:"url"`
}
