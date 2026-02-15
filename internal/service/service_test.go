package service_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/dsnikitin/shortener/internal/service"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetURL(ctx context.Context, id string) (models.URL, error) {
	return models.URL{}, nil
}
func (m *MockRepository) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	return nil, nil
}

func (m *MockRepository) PingDB(ctx context.Context) error               { return nil }
func (m *MockRepository) Save(ctx context.Context, url models.URL) error { return nil }
func (m *MockRepository) SaveMany(ctx context.Context, urls []models.URL) error {
	return m.Called().Error(0)
}
func (m *MockRepository) DeleteURLs(ctx context.Context, data []models.DeletableURL) {}
func (m *MockRepository) Close()                                                     {}

type MockURLDeleter struct {
	mock.Mock
}

func (m *MockURLDeleter) DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
	return m.Called().Error(0)
}

const (
	baseURL    = "https://example.com/"
	pathLength = 200
	batchSize  = 100000
)

func BenchmarkCreateIDs(b *testing.B) {
	b.Run("positive", func(b *testing.B) {
		b.StopTimer()

		userID := uuid.New()
		req := generateShortenBatchMap(batchSize)

		r := new(MockRepository)
		r.On("SaveMany").Return(nil).Once()

		s := service.New(r, new(MockURLDeleter))

		b.StartTimer()
		_, _ = s.CreateIDs(b.Context(), userID, req)
	})
	b.Run("already exists", func(b *testing.B) {
		b.StopTimer()

		err := &errx.ErrAlreadyExists{
			CorrelationID: strconv.Itoa(batchSize),
			URL:           models.URL{Original: baseURL},
		}

		userID := uuid.New()
		req := generateShortenBatchMap(batchSize)
		req[strconv.Itoa(batchSize)] = baseURL

		r := new(MockRepository)
		r.On("SaveMany").Return(err).Once()

		s := service.New(r, new(MockURLDeleter))

		b.StartTimer()
		_, _ = s.CreateIDs(b.Context(), userID, req)
	})
}

var pathTemplate = func() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789/-_"

	var sb strings.Builder
	sb.Grow(pathLength)

	for i := range pathLength {
		sb.WriteByte(chars[i%len(chars)])
	}

	return sb.String()
}()

func generateShortenBatchMap(n int) map[string]string {
	res := make(map[string]string, n)

	for i := range n {
		correlationID := strconv.Itoa(i)
		res[correlationID] = baseURL + pathTemplate
	}

	return res
}
