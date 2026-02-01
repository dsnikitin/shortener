package middleware

import (
	"net/http"
	"time"

	"github.com/dsnikitin/shortener/internal/logger"
)

// resData содержит информацию о статусе и размере ответа.
type resData struct {
	status int
	size   int
}

// loggingResWriter обертка для ResponseWriter для логирования.
type loggingResWriter struct {
	http.ResponseWriter
	res *resData
}

// Write записывает данные и отслеживает размер ответа.
func (w *loggingResWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.res.size += size
	return size, err
}

// WriteHeader устанавливает код статуса ответа.
func (w *loggingResWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.res.status = statusCode
}

// Logging middleware для логирования HTTP запросов и ответов.
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
