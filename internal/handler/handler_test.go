package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/dsnikitin/shortener/api/proto"
	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/middleware"
	"github.com/dsnikitin/shortener/internal/models"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) CreateID(ctx context.Context, userID uuid.UUID, url string) (string, error) {
	args := m.Called(userID, url)
	return args.String(0), args.Error(1)
}

func (m *MockService) CreateIDs(ctx context.Context, userID uuid.UUID, req map[string]string) (map[string]string, error) {
	args := m.Called(userID, req)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockService) GetURL(ctx context.Context, id string) (models.URL, error) {
	args := m.Called(id)
	return args.Get(0).(models.URL), args.Error(1)
}

func (m *MockService) PingDB(ctx context.Context) error {
	return m.Called().Error(0)
}

func (m *MockService) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.URL), args.Error(1)
}

func (m *MockService) DeleteUserURLs(ctx context.Context, userID uuid.UUID, ids []string) error {
	return m.Called(userID).Error(0)
}

func (m *MockService) GetStats(ctx context.Context) (models.Stats, error) {
	args := m.Called()
	return args.Get(0).(models.Stats), args.Error(1)
}

type MockAuditor struct {
	mock.Mock
}

func (m *MockAuditor) PublishEvent(models.Event) {}

func TestHTTPHandler_Shorten(t *testing.T) {
	userID := uuid.New()

	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resBody string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Post("/", h.Shorten)

	tests := []struct {
		name             string
		method           string
		userID           string
		reqBody          string
		servicesMockCall func()
		want             want
	}{
		{
			name:    "positive",
			method:  http.MethodPost,
			userID:  userID.String(),
			reqBody: "https://practicum.yandex.ru/",
			servicesMockCall: func() {
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("abcdefg", nil).Once()
			},
			want: want{
				code:    http.StatusCreated,
				headers: headers{contentType: "text/plain"},
				resBody: "http://localhost:8080/abcdefg",
			},
		},
		{
			name:             "wrong method",
			method:           http.MethodGet,
			reqBody:          "https://practicum.yandex.ru/",
			servicesMockCall: func() {},
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
		{
			name:             "empty body",
			method:           http.MethodPost,
			userID:           userID.String(),
			reqBody:          "",
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "empty body\n",
			},
		},
		{
			name:    "already exist",
			method:  http.MethodPost,
			userID:  userID.String(),
			reqBody: "https://practicum.yandex.ru/",
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru/"}
				err := errx.NewAlreadyExistsError(url, errors.New("already exists"))
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("", err).Once()
			},
			want: want{
				code:    http.StatusConflict,
				headers: headers{contentType: "text/plain"},
				resBody: "http://localhost:8080/abcdefg",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			req := httptest.NewRequest(test.method, "/", bytes.NewBufferString(test.reqBody))
			if test.userID != "" {
				req.Header.Set("x-user-id", test.userID)
			}
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()

			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			assert.Equal(t, test.want.resBody, string(resBody))
		})
	}
}

func ExampleHTTPHandler_Shorten() {
	userID := uuid.New()
	originalURL := "https://practicum.yandex.ru"

	service := new(MockService)
	service.On("CreateID", userID, originalURL).Return("abcdefg", nil).Once()

	h := handler.NewHTTPHandler("http://localhost:8080", service, new(MockAuditor))

	mux := chi.NewRouter()
	mux.Post("/", h.Shorten)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(originalURL))
	req.Header.Set("x-user-id", userID.String())

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer res.Body.Close()

	shortURL, _ := io.ReadAll(res.Body)
	fmt.Println(string(shortURL))

	// Output:
	// http://localhost:8080/abcdefg
}

func TestHTTPHandler_ShortenFromJSON(t *testing.T) {
	userID := uuid.New()

	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resp    models.ShortenResponse
		errMsg  string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Post("/api/shorten", h.ShortenFromJSON)

	tests := []struct {
		name             string
		method           string
		userID           string
		req              models.ShortenRequest
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive",
			method: http.MethodPost,
			userID: userID.String(),
			req:    models.ShortenRequest{URL: "https://practicum.yandex.ru/"},
			servicesMockCall: func() {
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("abcdefg", nil).Once()
			},
			want: want{
				code:    http.StatusCreated,
				headers: headers{contentType: "application/json"},
				resp:    models.ShortenResponse{Result: "http://localhost:8080/abcdefg"},
			},
		},
		{
			name:             "wrong method",
			method:           http.MethodGet,
			req:              models.ShortenRequest{URL: "https://practicum.yandex.ru/"},
			servicesMockCall: func() {},
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
		{
			name:             "empty url",
			method:           http.MethodPost,
			userID:           userID.String(),
			req:              models.ShortenRequest{URL: ""},
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "empty url\n",
			},
		},
		{
			name:   "already exist",
			method: http.MethodPost,
			userID: userID.String(),
			req:    models.ShortenRequest{URL: "https://practicum.yandex.ru/"},
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru/"}
				err := errx.NewAlreadyExistsError(url, errors.New("already exists"))
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("", err).Once()
			},
			want: want{
				code:    http.StatusConflict,
				headers: headers{contentType: "application/json"},
				resp:    models.ShortenResponse{Result: "http://localhost:8080/abcdefg"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			body, err := json.Marshal(test.req)
			require.NoError(t, err)

			req := httptest.NewRequest(test.method, "/api/shorten", bytes.NewBuffer(body))
			req.Header.Add("Content-Type", "application/json")
			if test.userID != "" {
				req.Header.Set("x-user-id", test.userID)
			}
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			if res.StatusCode >= 400 && res.StatusCode != 409 {
				resp, readErr := io.ReadAll(res.Body)
				require.NoError(t, readErr)
				assert.Equal(t, test.want.errMsg, string(resp))
			} else {
				resp := models.ShortenResponse{}
				err = json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, test.want.resp, resp)
			}
		})
	}
}

func ExampleHTTPHandler_ShortenFromJSON() {
	userID := uuid.New()
	req := models.ShortenRequest{URL: "https://practicum.yandex.ru"}

	service := new(MockService)
	service.On("CreateID", userID, req.URL).Return("abcdefg", nil).Once()

	h := handler.NewHTTPHandler("http://localhost:8080", service, new(MockAuditor))

	mux := chi.NewRouter()
	mux.Post("/api/shorten", h.ShortenFromJSON)

	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBuffer(body))
	httpReq.Header.Set("x-user-id", userID.String())
	httpReq.Header.Add("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httpReq)

	res := recorder.Result()
	defer res.Body.Close()

	var resp models.ShortenResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp.Result)

	// Output:
	// http://localhost:8080/abcdefg
}

func TestHTTPHandler_Redirect(t *testing.T) {
	type headers struct {
		contentType string
		location    string
	}
	type want struct {
		code    int
		headers headers
		resBody string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Get("/{id}", h.Redirect)

	tests := []struct {
		name             string
		id               string
		method           string
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive",
			id:     "abcdefg",
			method: http.MethodGet,
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru"}
				s.On("GetURL", "abcdefg").Return(url, nil).Once()
			},
			want: want{
				code: http.StatusTemporaryRedirect,
				headers: headers{
					location:    "https://practicum.yandex.ru",
					contentType: "text/plain",
				},
			},
		},
		{
			name:             "empty id",
			id:               "",
			method:           http.MethodGet,
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusNotFound,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "404 page not found\n",
			},
		},
		{
			name:             "too large id",
			id:               "abcdefghi",
			method:           http.MethodGet,
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "incorrect id\n",
			},
		},
		{
			name:   "not found",
			id:     "gfedcba",
			method: http.MethodGet,
			servicesMockCall: func() {
				s.On("GetURL", "gfedcba").Return(models.URL{}, errx.ErrNotFound).Once()
			},
			want: want{
				code:    http.StatusNotFound,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "id not found\n",
			},
		},
		{
			name:   "gone",
			id:     "abcdefg",
			method: http.MethodGet,
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru", IsDeleted: true}
				s.On("GetURL", "abcdefg").Return(url, nil).Once()
			},
			want: want{
				code:    http.StatusGone,
				headers: headers{contentType: "text/plain"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			req := httptest.NewRequest(test.method, "/"+test.id, nil)
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()

			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.location, res.Header.Get("Location"))
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			assert.Equal(t, test.want.resBody, string(resBody))
		})
	}
}

func ExampleHTTPHandler_Redirect() {
	short := "abcdefg"
	url := models.URL{ID: short, Original: "https://practicum.yandex.ru"}

	service := new(MockService)
	service.On("GetURL", "abcdefg").Return(url, nil).Once()

	h := handler.NewHTTPHandler("http://localhost:8080", service, new(MockAuditor))

	mux := chi.NewRouter()
	mux.Get("/{id}", h.Redirect)

	req := httptest.NewRequest(http.MethodGet, "/"+short, nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer res.Body.Close()

	fmt.Println(res.Header.Get("Location"))

	// Output:
	// https://practicum.yandex.ru
}

func TestHTTPHandler_PingDB(t *testing.T) {
	type want struct {
		code        int
		contentType string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Get("/ping", h.PingDB)

	tests := []struct {
		name             string
		method           string
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive",
			method: http.MethodGet,
			servicesMockCall: func() {
				s.On("PingDB").Return(nil).Once()
			},
			want: want{
				code:        http.StatusOK,
				contentType: "text/plain",
			},
		},
		{
			name:             "wrong method",
			method:           http.MethodPost,
			servicesMockCall: func() {},
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
		{
			name:   "negative",
			method: http.MethodGet,
			servicesMockCall: func() {
				s.On("PingDB").Return(errors.New("some error")).Once()
			},
			want: want{
				code:        http.StatusInternalServerError,
				contentType: "text/plain; charset=utf-8",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			req := httptest.NewRequest(test.method, "/ping", nil)
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))
		})
	}
}

func TestHTTPHandler_ShortenBatch(t *testing.T) {
	userID := uuid.New()

	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resp    []models.ShortenBatchResponse
		errMsg  string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Post("/api/shorten/batch", h.ShortenBatch)

	tests := []struct {
		name             string
		method           string
		userID           string
		req              []models.ShortenBatchRequest
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive test",
			method: http.MethodPost,
			userID: userID.String(),
			req: []models.ShortenBatchRequest{
				{CorrelationID: "1", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
				{CorrelationID: "2", OriginalURL: "https://practicum.yandex.ru/"},
			},
			servicesMockCall: func() {
				URLs := map[string]string{"1": "http://bstoudwr.biz/ray1dv90xyg", "2": "https://practicum.yandex.ru/"}
				ids := map[string]string{"1": "gfedcba", "2": "abcdefg"}

				s.On("CreateIDs", userID, URLs).Return(ids, nil).Once()
			},
			want: want{
				code:    http.StatusCreated,
				headers: headers{contentType: "application/json"},
				resp: []models.ShortenBatchResponse{
					{CorrelationID: "1", ShortURL: "http://localhost:8080/gfedcba"},
					{CorrelationID: "2", ShortURL: "http://localhost:8080/abcdefg"},
				},
			},
		},
		{
			name:   "wrong method",
			method: http.MethodGet,
			req: []models.ShortenBatchRequest{
				{CorrelationID: "1", OriginalURL: "https://practicum.yandex.ru/"},
				{CorrelationID: "2", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
			},
			servicesMockCall: func() {},
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
		{
			name:             "empty request",
			method:           http.MethodPost,
			userID:           userID.String(),
			req:              []models.ShortenBatchRequest{},
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "empty request\n",
			},
		},
		{
			name:   "duplicate correlationID",
			method: http.MethodPost,
			userID: userID.String(),
			req: []models.ShortenBatchRequest{
				{CorrelationID: "1", OriginalURL: "https://practicum.yandex.ru/"},
				{CorrelationID: "2", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
				{CorrelationID: "1", OriginalURL: "http://sbi6ruqq502jq.ru"},
			},
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "duplicate correlationID 1\n",
			},
		},
		{
			name:   "already exist",
			method: http.MethodPost,
			userID: userID.String(),
			req: []models.ShortenBatchRequest{
				{CorrelationID: "1", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
				{CorrelationID: "2", OriginalURL: "https://practicum.yandex.ru/"},
			},
			servicesMockCall: func() {
				URLs := map[string]string{"1": "http://bstoudwr.biz/ray1dv90xyg", "2": "https://practicum.yandex.ru/"}

				existingURL := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru/"}
				err := &errx.ErrAlreadyExists{CorrelationID: "2", URL: existingURL, Err: errors.New("already exists")}
				s.On("CreateIDs", userID, URLs).Return(map[string]string{}, err).Once()
			},
			want: want{
				code:    http.StatusConflict,
				headers: headers{contentType: "application/json"},
				resp:    []models.ShortenBatchResponse{{CorrelationID: "2", ShortURL: "http://localhost:8080/abcdefg"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			body, err := json.Marshal(test.req)
			require.NoError(t, err)

			req := httptest.NewRequest(test.method, "/api/shorten/batch", bytes.NewBuffer(body))
			req.Header.Add("Content-Type", "application/json")
			if test.userID != "" {
				req.Header.Set("x-user-id", test.userID)
			}
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			if res.StatusCode >= 400 && res.StatusCode != 409 {
				resp, readErr := io.ReadAll(res.Body)
				require.NoError(t, readErr)
				assert.Equal(t, test.want.errMsg, string(resp))
			} else {
				resp := []models.ShortenBatchResponse{}
				err = json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, test.want.resp, resp)
			}
		})
	}
}

func ExampleHTTPHandler_ShortenBatch() {
	userID := uuid.New()
	req := []models.ShortenBatchRequest{
		{CorrelationID: "1", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
		{CorrelationID: "2", OriginalURL: "https://practicum.yandex.ru"},
	}

	service := new(MockService)
	URLs := map[string]string{"1": "http://bstoudwr.biz/ray1dv90xyg", "2": "https://practicum.yandex.ru"}
	ids := map[string]string{"1": "gfedcba", "2": "abcdefg"}
	service.On("CreateIDs", userID, URLs).Return(ids, nil).Once()

	h := handler.NewHTTPHandler("http://localhost:8080", service, new(MockAuditor))

	mux := chi.NewRouter()
	mux.Post("/api/shorten/batch", h.ShortenBatch)

	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(body))
	httpReq.Header.Set("x-user-id", userID.String())
	httpReq.Header.Add("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httpReq)

	res := recorder.Result()
	defer res.Body.Close()

	var resp []models.ShortenBatchResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(resp[0].ShortURL)
	fmt.Println(resp[1].ShortURL)

	// Output:
	// http://localhost:8080/gfedcba
	// http://localhost:8080/abcdefg
}

func TestHTTPHandler_GetUserURLs(t *testing.T) {
	userID := uuid.New()

	type headers struct {
		contentType string
		userID      string
	}

	type want struct {
		code    int
		headers headers
		resp    []models.GetUserUrlsResponseItem
		errMsg  string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.NewHTTPHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))

	r := chi.NewRouter()
	r.Get("/api/user/urls", h.GetUserURLs)

	tests := []struct {
		name             string
		method           string
		userID           string
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive with urls",
			method: http.MethodGet,
			userID: userID.String(),
			servicesMockCall: func() {
				urls := []models.URL{
					{ID: "gfedcba", Original: "http://bstoudwr.biz/ray1dv90xyg"},
					{ID: "abcdefg", Original: "https://practicum.yandex.ru/"},
				}
				s.On("GetUserURLs", userID).Return(urls, nil).Once()
			},
			want: want{
				code:    http.StatusOK,
				headers: headers{contentType: "application/json"},
				resp: []models.GetUserUrlsResponseItem{
					{ShortURL: cfg.ShortURLBaseAddr + "/" + "gfedcba", OriginalURL: "http://bstoudwr.biz/ray1dv90xyg"},
					{ShortURL: cfg.ShortURLBaseAddr + "/" + "abcdefg", OriginalURL: "https://practicum.yandex.ru/"},
				},
			},
		},
		{
			name:   "positive no content",
			method: http.MethodGet,
			userID: userID.String(),
			servicesMockCall: func() {
				s.On("GetUserURLs", userID).Return([]models.URL{}, nil).Once()
			},
			want: want{
				code:    http.StatusNoContent,
				headers: headers{contentType: "application/json"},
			},
		},
		{
			name:             "wrong method",
			method:           http.MethodPost,
			userID:           userID.String(),
			servicesMockCall: func() {},
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
		{
			name:             "missing x-user-id header",
			method:           http.MethodGet,
			userID:           "",
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusInternalServerError,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "internal server error\n",
			},
		},
		{
			name:             "invalid userID",
			method:           http.MethodGet,
			userID:           "invalid-uuid",
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusInternalServerError,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "internal server error\n",
			},
		},
		{
			name:   "service error",
			method: http.MethodGet,
			userID: userID.String(),
			servicesMockCall: func() {
				s.On("GetUserURLs", userID).Return(nil, errors.New("database error")).Once()
			},
			want: want{
				code:    http.StatusInternalServerError,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "internal server error\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			req := httptest.NewRequest(test.method, "/api/user/urls", nil)
			if test.userID != "" {
				req.Header.Set("x-user-id", test.userID)
			}
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))

			switch {
			case test.want.code == http.StatusOK:
				var resp []models.GetUserUrlsResponseItem
				err := json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, test.want.resp, resp)
			case test.want.code == http.StatusNoContent:
				assert.Equal(t, 0, recorder.Body.Len(), "response body should be empty for 204 status code")
			case test.want.code >= 400:
				respBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, test.want.errMsg, string(respBody))

			}
		})
	}
}

func ExampleHTTPHandler_GetUserURLs() {
	userID := uuid.New()

	service := new(MockService)
	urls := []models.URL{
		{ID: "gfedcba", Original: "http://bstoudwr.biz/ray1dv90xyg"},
		{ID: "abcdefg", Original: "https://practicum.yandex.ru"},
	}
	service.On("GetUserURLs", userID).Return(urls, nil).Once()

	h := handler.NewHTTPHandler("http://localhost:8080", service, new(MockAuditor))

	mux := chi.NewRouter()
	mux.Get("/api/user/urls", h.GetUserURLs)

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req.Header.Set("x-user-id", userID.String())

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer res.Body.Close()

	var resp []models.GetUserUrlsResponseItem
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s %s\n", resp[0].ShortURL, resp[0].OriginalURL)
	fmt.Printf("%s %s\n", resp[1].ShortURL, resp[1].OriginalURL)

	// Output:
	// http://localhost:8080/gfedcba http://bstoudwr.biz/ray1dv90xyg
	// http://localhost:8080/abcdefg https://practicum.yandex.ru
}

func TestGRPCHandler_ShortenURL(t *testing.T) {
	userID := uuid.New()

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
		JWTSigningKey:    "some-secret-key",
	}

	s := new(MockService)
	h := handler.NewGRPCHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))
	serverStopFn, serverAddr := startGRPCServer(t, cfg.JWTSigningKey, h)
	defer serverStopFn()

	client, conn := initGRPCClient(t, serverAddr)
	defer conn.Close()

	md := initMetaData(t, cfg.JWTSigningKey, userID)

	type want struct {
		code   codes.Code
		result string
	}

	tests := []struct {
		name             string
		userID           string
		reqBody          string
		servicesMockCall func()
		want             want
	}{
		{
			name:    "positive",
			userID:  userID.String(),
			reqBody: "https://practicum.yandex.ru/",
			servicesMockCall: func() {
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("abcdefg", nil).Once()
			},
			want: want{
				code:   codes.OK,
				result: "http://localhost:8080/abcdefg",
			},
		},
		{
			name:             "empty body",
			userID:           userID.String(),
			reqBody:          "",
			servicesMockCall: func() {},
			want: want{
				code:   codes.InvalidArgument,
				result: "empty url",
			},
		},
		{
			name:    "already exist",
			userID:  userID.String(),
			reqBody: "https://practicum.yandex.ru/",
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru/"}
				err := errx.NewAlreadyExistsError(url, errors.New("already exists"))
				s.On("CreateID", userID, "https://practicum.yandex.ru/").Return("", err).Once()
			},
			want: want{
				code:   codes.AlreadyExists,
				result: "http://localhost:8080/abcdefg",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			ctx := metadata.NewOutgoingContext(context.Background(), md)
			resp, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{
				Url: test.reqBody,
			}.Build())

			statusCode := status.Code(err)
			assert.Equal(t, test.want.code, statusCode)

			switch statusCode {
			case codes.OK:
				assert.Equal(t, test.want.result, resp.GetResult())
			default:
				assert.Equal(t, test.want.result, status.Convert(err).Message())
			}
		})
	}
}

func TestGRPCHandler_ExpandURL(t *testing.T) {
	userID := uuid.New()

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
		JWTSigningKey:    "some-secret-key",
	}

	s := new(MockService)
	h := handler.NewGRPCHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))
	serverStopFn, serverAddr := startGRPCServer(t, cfg.JWTSigningKey, h)
	defer serverStopFn()

	client, conn := initGRPCClient(t, serverAddr)
	defer conn.Close()

	md := initMetaData(t, cfg.JWTSigningKey, userID)

	type want struct {
		code   codes.Code
		result string
	}

	tests := []struct {
		name             string
		urlID            string
		reqBody          string
		servicesMockCall func()
		want             want
	}{
		{
			name:  "positive",
			urlID: "abcdefg",
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru"}
				s.On("GetURL", "abcdefg").Return(url, nil).Once()
			},
			want: want{
				code:   codes.OK,
				result: "https://practicum.yandex.ru",
			},
		},
		{
			name:             "empty id",
			urlID:            "",
			servicesMockCall: func() {},
			want: want{
				code:   codes.InvalidArgument,
				result: "incorrect id",
			},
		},
		{
			name:             "too large id",
			urlID:            "abcdefghi",
			servicesMockCall: func() {},
			want: want{
				code:   codes.InvalidArgument,
				result: "incorrect id",
			},
		},
		{
			name:  "not found",
			urlID: "gfedcba",
			servicesMockCall: func() {
				s.On("GetURL", "gfedcba").Return(models.URL{}, errx.ErrNotFound).Once()
			},
			want: want{
				code:   codes.NotFound,
				result: "id not found",
			},
		},
		{
			name:  "gone",
			urlID: "abcdefg",
			servicesMockCall: func() {
				url := models.URL{ID: "abcdefg", Original: "https://practicum.yandex.ru", IsDeleted: true}
				s.On("GetURL", "abcdefg").Return(url, nil).Once()
			},
			want: want{
				code:   codes.FailedPrecondition,
				result: "url is deleted",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			ctx := metadata.NewOutgoingContext(context.Background(), md)
			resp, err := client.ExpandURL(ctx, pb.URLExpandRequest_builder{
				Id: test.urlID,
			}.Build())

			statusCode := status.Code(err)
			assert.Equal(t, test.want.code, statusCode)

			switch statusCode {
			case codes.OK:
				assert.Equal(t, test.want.result, resp.GetResult())
			default:
				assert.Equal(t, test.want.result, status.Convert(err).Message())
			}
		})
	}
}

func TestGRPCHandler_ListUserURLs(t *testing.T) {
	userID := uuid.New()

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
		JWTSigningKey:    "some-secret-key",
	}

	s := new(MockService)
	h := handler.NewGRPCHandler(cfg.ShortURLBaseAddr, s, new(MockAuditor))
	serverStopFn, serverAddr := startGRPCServer(t, cfg.JWTSigningKey, h)
	defer serverStopFn()

	client, conn := initGRPCClient(t, serverAddr)
	defer conn.Close()

	md := initMetaData(t, cfg.JWTSigningKey, userID)

	type want struct {
		code   codes.Code
		errMsg string
		result []*pb.URLData
	}

	tests := []struct {
		name             string
		userID           string
		reqBody          string
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive with urls",
			userID: userID.String(),
			servicesMockCall: func() {
				urls := []models.URL{
					{ID: "gfedcba", Original: "http://bstoudwr.biz/ray1dv90xyg"},
					{ID: "abcdefg", Original: "https://practicum.yandex.ru/"},
				}
				s.On("GetUserURLs", userID).Return(urls, nil).Once()
			},
			want: want{
				code: codes.OK,
				result: []*pb.URLData{
					pb.URLData_builder{
						ShortUrl:    cfg.ShortURLBaseAddr + "/" + "gfedcba",
						OriginalUrl: "http://bstoudwr.biz/ray1dv90xyg",
					}.Build(),
					pb.URLData_builder{
						ShortUrl:    cfg.ShortURLBaseAddr + "/" + "abcdefg",
						OriginalUrl: "https://practicum.yandex.ru/",
					}.Build(),
				},
			},
		},
		{
			name:   "positive no content",
			userID: userID.String(),
			servicesMockCall: func() {
				s.On("GetUserURLs", userID).Return([]models.URL{}, nil).Once()
			},
			want: want{
				code:   codes.OK,
				result: []*pb.URLData(nil),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			ctx := metadata.NewOutgoingContext(context.Background(), md)
			resp, err := client.ListUserURLs(ctx, &emptypb.Empty{})

			statusCode := status.Code(err)
			assert.Equal(t, test.want.code, statusCode)

			switch statusCode {
			case codes.OK:
				assert.Equal(t, test.want.result, resp.GetUrl())
			default:
				assert.Equal(t, test.want.result, status.Convert(err).Message())
			}
		})
	}
}

func startGRPCServer(t *testing.T, jwtSigningKey string, h pb.ShortenerServiceServer) (func(), string) {
	server := grpc.NewServer(grpc.UnaryInterceptor(middleware.AuthInterceptor(jwtSigningKey)))
	pb.RegisterShortenerServiceServer(server, h)

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	go func() {
		if err = server.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			t.Logf("server stopped: %v", err)
		}
	}()

	return server.GracefulStop, listener.Addr().String()
}

func initGRPCClient(t *testing.T, serverAddr string) (pb.ShortenerServiceClient, *grpc.ClientConn) {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return pb.NewShortenerServiceClient(conn), conn
}

func initMetaData(t *testing.T, jwtSigningKey string, userID uuid.UUID) metadata.MD {
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, struct {
		jwt.RegisteredClaims
		UserID uuid.UUID
	}{
		UserID: userID,
	})

	signedToken, err := jwtToken.SignedString([]byte(jwtSigningKey))
	require.NoError(t, err)

	return metadata.New(map[string]string{"authorization": "Bearer " + signedToken})
}
