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
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) CreateID(url string) (string, error) {
	args := m.Called(url)
	return args.String(0), args.Error(1)
}

func (m *MockService) GetOriginal(id string) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockService) PingDB() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandler_Shorten(t *testing.T) {
	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resBody string
	}

	conf := &config.Config{
		ServerAddr:       "localhost:8080",
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.New(conf, s)

	r := chi.NewRouter()
	r.Post("/", h.Shorten)

	tests := []struct {
		name             string
		method           string
		reqBody          string
		servicesMockCall func()
		want             want
	}{
		{
			name:    "positive test",
			method:  http.MethodPost,
			reqBody: "https://practicum.yandex.ru/",
			servicesMockCall: func() {
				s.On("CreateID", "https://practicum.yandex.ru/").Return("abcdefg", nil).Once()
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
			reqBody:          "",
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "empty body\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.servicesMockCall()

			req := httptest.NewRequest(test.method, "/", bytes.NewBufferString(test.reqBody))
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
	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resp    models.ShortenResponse
		errMsg  string
	}

	conf := &config.Config{
		ServerAddr:       "localhost:8080",
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.New(conf, s)

	r := chi.NewRouter()
	r.Post("/api/shorten", h.ShortenFromJSON)

	tests := []struct {
		name             string
		method           string
		req              models.ShortenRequest
		servicesMockCall func()
		want             want
	}{
		{
			name:   "positive test",
			method: http.MethodPost,
			req:    models.ShortenRequest{URL: "https://practicum.yandex.ru/"},
			servicesMockCall: func() {
				s.On("CreateID", "https://practicum.yandex.ru/").Return("abcdefg", nil).Once()
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
			req:              models.ShortenRequest{URL: ""},
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				errMsg:  "empty url\n",
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
			recorder := httptest.NewRecorder()

			r.ServeHTTP(recorder, req)
			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			if res.StatusCode >= 400 {
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

	conf := &config.Config{
		ServerAddr:       "localhost:8080",
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.New(conf, s)

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
				code:    http.StatusBadRequest,
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

	conf := &config.Config{
		ServerAddr:       "localhost:8080",
		ShortURLBaseAddr: "http://localhost:8080",
	}

	s := new(MockService)
	h := handler.New(conf, s)

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
