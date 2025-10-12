package handler_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsnikitin/shortener/internal/handler"
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

func TestHandler_Shorten(t *testing.T) {
	type headers struct {
		contentType string
	}

	type want struct {
		code    int
		headers headers
		resBody string
	}

	s := new(MockService)
	h := handler.New(s)

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
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "only POST requests are allowed\n",
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
			req := httptest.NewRequest(test.method, "/", bytes.NewBufferString(test.reqBody))
			test.servicesMockCall()

			recoder := httptest.NewRecorder()
			h.Shorten(recoder, req)
			res := recoder.Result()

			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.headers.contentType, res.Header.Get("Content-Type"))
			assert.Equal(t, test.want.resBody, string(resBody))
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

	s := new(MockService)
	h := handler.New(s)

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
			name:             "wrong method",
			id:               "abcdefg",
			method:           http.MethodPost,
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "оnly GET requests are allowed\n",
			},
		},
		{
			name:             "empty id",
			id:               "",
			method:           http.MethodGet,
			servicesMockCall: func() {},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "id is required\n",
			},
		},
		{
			name:   "id not found",
			id:     "gfedcba",
			method: http.MethodGet,
			servicesMockCall: func() {
				s.On("GetOriginal", "gfedcba").Return("", errors.New("url not found")).Once()
			},
			want: want{
				code:    http.StatusBadRequest,
				headers: headers{contentType: "text/plain; charset=utf-8"},
				resBody: "url not found\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "/"+test.id, nil)
			test.servicesMockCall()

			recoder := httptest.NewRecorder()
			h.Redirect(recoder, req)
			res := recoder.Result()

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
