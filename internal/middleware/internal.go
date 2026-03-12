package middleware

import (
	"net"
	"net/http"
	"strings"
)

// SubnetGuard middleware для проверки ip адреса клиента для допуска к внутренним эндпоинтам.
func SubnetGuard(trustedSubnet *net.IPNet) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedSubnet == nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			ip := net.ParseIP(getClientIP(r))
			if ip == nil || !trustedSubnet.Contains(ip) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	if ff := r.Header.Get("X-Forwarded-For"); ff != "" {
		if ips := strings.Split(ff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	return ""
}
