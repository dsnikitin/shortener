package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/dsnikitin/shortener/internal/service/deleter"
	"github.com/google/uuid"
)

type Repository interface {
	PingDB(ctx context.Context) error
	Save(ctx context.Context, url models.URL) error
	SaveMany(ctx context.Context, urls []models.URL) error
	GetURL(ctx context.Context, id string) (models.URL, error)
	GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error)
	DeleteURLs(ctx context.Context, data []models.DeletableURL)
	Close()
}

type URLDeleter interface {
	DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error
	Run()
	Stop()
}

type Service struct {
	r Repository
	d URLDeleter
}

func New(r Repository) *Service {
	s := &Service{
		r: r,
		d: deleter.New(r),
	}

	s.d.Run()

	return s
}

func (s *Service) CreateID(ctx context.Context, userID uuid.UUID, originalURL string) (string, error) {
	url := models.URL{
		ID:        generateID(originalURL),
		Original:  originalURL,
		CreatorID: userID,
	}

	if err := s.r.Save(ctx, url); err != nil {
		var aeErr *errx.ErrAlreadyExists
		if errors.As(err, &aeErr) {
			if url.Original != aeErr.URL.Original {
				return "", fmt.Errorf("collision detected - common id %s for requested url %s and existing url %s",
					aeErr.URL.ID, url.Original, aeErr.URL.Original)
			}
		}

		return "", fmt.Errorf("save: %w", err)
	}

	return url.ID, nil
}

func (s *Service) CreateIDs(ctx context.Context, userID uuid.UUID, req map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(req))
	urls := make([]models.URL, 0, len(req))

	for correlationID, originalURL := range req {
		id := generateID(originalURL)

		result[correlationID] = id

		urls = append(urls, models.URL{
			ID:        id,
			Original:  originalURL,
			CreatorID: userID,
		})
	}

	if err := s.r.SaveMany(ctx, urls); err != nil {
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
						id, originalURL, aeErr.URL.Original)
				}

			}

		}

		return nil, fmt.Errorf("save: %w", err)
	}

	return result, nil
}

func (s *Service) GetURL(ctx context.Context, id string) (models.URL, error) {
	return s.r.GetURL(ctx, id)
}

func (s *Service) PingDB(ctx context.Context) error {
	return s.r.PingDB(ctx)
}

func (s *Service) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	return s.r.GetUserURLs(ctx, userID)
}

func (s *Service) DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
	return s.d.DeleteUserURLs(ctx, userID, ids)
}

func (s *Service) Stop() {
	s.d.Stop()
	s.r.Close()
}

func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	id := base64.URLEncoding.EncodeToString(hash[:config.IDMaxLength])[:config.IDMaxLength]

	return strings.TrimRight(id, "=")
}
