package service

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/dsnikitin/shortener/internal/model"
)

type Repository interface {
	Save(url *model.URL) error
	Get(id string) (*model.URL, error)
}

type Service struct {
	r Repository
}

func New(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) CreateID(original string) (string, error) {
	url := &model.URL{
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
	id := base64.URLEncoding.EncodeToString(hash[:8])[:8]

	return strings.TrimRight(id, "=")
}
