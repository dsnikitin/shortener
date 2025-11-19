package service

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/models"
)

type Repository interface {
	Save(url models.URL) error
	SaveMany(urls []models.URL) error
	Get(id string) (models.URL, error)
	PingDB() error
	Close()
}

type Service struct {
	r Repository
}

func New(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) CreateID(original string) (string, error) {
	url := models.URL{
		ID:       generateID(original),
		Original: original,
	}

	if err := s.r.Save(url); err != nil {
		var aeErr *errx.ErrAlreadyExists
		if errors.As(err, &aeErr) {
			if url.Original != aeErr.URL.Original {
				return "", fmt.Errorf("collision detected - common id %s for requested url %s and existing url %s",
					aeErr.URL.ID, url.Original, aeErr.URL)
			}
		}

		return "", fmt.Errorf("save: %w", err)
	}

	return url.ID, nil
}

func (s *Service) CreateIDs(req map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(req))
	urls := make([]models.URL, 0, len(req))

	for correlationID, originalURL := range req {
		id := generateID(originalURL)

		result[correlationID] = id

		urls = append(urls, models.URL{
			ID:       id,
			Original: originalURL,
		})
	}

	if err := s.r.SaveMany(urls); err != nil {
		var aeErr *errx.ErrAlreadyExists
		if errors.As(err, &aeErr) {
			for correlationID, id := range result {
				originalURL := req[correlationID]

				if originalURL == aeErr.URL.Original {
					aeErr.CorrelationID = correlationID
					return nil, fmt.Errorf("save: %w", err)
				}

				if id == aeErr.URL.ID {
					return nil, fmt.Errorf("collision detected - common id %s for requested url %s and existing url %s",
						id, originalURL, aeErr.URL)
				}

			}

		}

		return nil, fmt.Errorf("save: %w", err)
	}

	return result, nil
}

func (s *Service) GetOriginal(id string) (string, error) {
	url, err := s.r.Get(id)
	if err != nil {
		return "", err
	}

	return url.Original, nil
}

func (s *Service) PingDB() error {
	return s.r.PingDB()
}

func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	id := base64.URLEncoding.EncodeToString(hash[:config.IDMaxLength])[:config.IDMaxLength]

	return strings.TrimRight(id, "=")
}
