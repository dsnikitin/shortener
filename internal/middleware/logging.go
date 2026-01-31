package middleware

import (
	"net/http"
	"time"

	"github.com/dsnikitin/shortener/internal/logger"
)

type resData struct {
	status int
	size   int
}

type loggingResWriter struct {
	http.ResponseWriter
	res *resData
}

func (w *loggingResWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.res.size += size
	return size, err
}

func (w *loggingResWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.res.status = statusCode
}

func Logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lw := loggingResWriter{
			ResponseWriter: w,
			res:            &resData{},
		}

		h.ServeHTTP(&lw, r)

		logger.Log.Infow("Request handled",
			"uri", r.RequestURI,
			"method", r.Method,
			"status", lw.res.status,
			"duration", time.Since(start),
			"size", lw.res.size,
		)
	})
}
