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
	SaveMany(urls []models.URL) error
	Get(id string) (*models.URL, error)
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
	url := &models.URL{
		ID:       generateID(original),
		Original: original,
	}

	if err := s.r.Save(url); err != nil {
		return "", err
	}

	return url.ID, nil
}

func (s *Service) CreateIDs(reqs []models.ShortenBatchRequest) (map[string]string, error) {
	res := make(map[string]string, len(reqs))
	urls := make([]models.URL, 0, len(reqs))

	for _, req := range reqs {
		id := generateID(req.OriginalURL)

		res[req.CorrelationID] = id

		urls = append(urls, models.URL{
			ID:       id,
			Original: req.OriginalURL,
		})
	}

	if err := s.r.SaveMany(urls); err != nil {
		return nil, err
	}

	return res, nil
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
