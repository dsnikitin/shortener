package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) CreateID(userID uuid.UUID, url string) (string, error) {
	args := m.Called(userID, url)
	return args.String(0), args.Error(1)
}

func (m *MockService) CreateIDs(userID uuid.UUID, req map[string]string) (map[string]string, error) {
	args := m.Called(userID, req)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockService) GetOriginal(id string) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockService) PingDB() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockService) GetUserURLs(userID uuid.UUID) ([]models.URL, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.URL), args.Error(1)
}

func TestHandler_Shorten(t *testing.T) {
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
	h := handler.New(cfg.ShortURLBaseAddr, s)

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

func TestHandler_ShortenFromJSON(t *testing.T) {
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
	h := handler.New(cfg.ShortURLBaseAddr, s)

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
				resp, err := io.ReadAll(res.Body)
				require.NoError(t, err)
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

func TestHandler_Redirect(t *testing.T) {
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
	h := handler.New(cfg.ShortURLBaseAddr, s)

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
				s.On("GetOriginal", "abcdefg").Return("http://localhost:8080/abcdefg", nil).Once()
			},
			want: want{
				code: http.StatusTemporaryRedirect,
				headers: headers{
					location:    "http://localhost:8080/abcdefg",
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
				s.On("GetOriginal", "gfedcba").Return("", errors.New("id not found")).Once()
			},
			want: want{
				code:    http.StatusNotFound,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "id not found\n",
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

func TestHandler_PingDB(t *testing.T) {
	type want struct {
		code        int
		contentType string
	}

	cfg := &config.Config{
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.New(cfg.ShortURLBaseAddr, s)

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

func TestHandler_ShortenBatch(t *testing.T) {
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
	h := handler.New(cfg.ShortURLBaseAddr, s)

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
				resp, err := io.ReadAll(res.Body)
				require.NoError(t, err)
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

func TestHandler_GetUserUrls(t *testing.T) {
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
	h := handler.New(cfg.ShortURLBaseAddr, s)

	r := chi.NewRouter()
	r.Get("/api/user/urls", h.GetUserUrls)

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

			if test.want.code == http.StatusOK {
				var resp []models.GetUserUrlsResponseItem
				err := json.NewDecoder(res.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, test.want.resp, resp)
			} else if test.want.code == http.StatusNoContent {
				assert.Equal(t, 0, recorder.Body.Len(), "response body should be empty for 204 status code")
			} else if test.want.code >= 400 {
				respBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Equal(t, test.want.errMsg, string(respBody))
			}
		})
	}
}
