package models

import "github.com/google/uuid"

type URL struct {
	ID        string
	Original  string
	CreatorID uuid.UUID
	IsDeleted bool
}

type DeletableURL struct {
	ID        string
	CreatorID uuid.UUID
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

type ShortenBatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type ShortenBatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

type GetUserUrlsResponseItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type Action string

const (
	Shorten Action = "shorten"
	Follow  Action = "follow"
)

type Event struct {
	Timestamp   int64  `json:"ts"`
	Action      Action `json:"action"`
	UserID      string `json:"user_id,omitempty"`
	OriginalURL string `json:"url"`
}
