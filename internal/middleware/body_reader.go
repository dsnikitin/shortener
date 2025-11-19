package middleware

import (
	"net/http"
)

func BodyMaxBytesReader(maxSize int64) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)

			h.ServeHTTP(w, r)
		})
	}
}
