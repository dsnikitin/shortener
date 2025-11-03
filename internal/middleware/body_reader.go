package middleware

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
)

func BodyMaxBytesReader(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, config.OriginalURLMaxLength)

		h.ServeHTTP(w, r)
	})
}
