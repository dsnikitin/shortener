package service

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/models"
)

type Repository interface {
	Save(url *models.URL) error
	Get(id string) (*models.URL, error)
}

type Service struct {
	r Repository
}

func New(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) CreateID(original string) (string, error) {
	url := &models.URL{
		ID:       generateID(original),
		Original: original,
	}

	if err := s.r.Save(url); err != nil {
		return "", err
	}

	return url.ID, nil
}

func (s *Service) GetOriginal(id string) (string, error) {
	url, err := s.r.Get(id)
	if err != nil {
		return "", err
	}

	return url.Original, nil
}

func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	id := base64.URLEncoding.EncodeToString(hash[:config.IDMaxLength])[:config.IDMaxLength]

	return strings.TrimRight(id, "=")
}
