package middleware

import (
	"net/http"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dsnikitin/shortener/internal/logger"
)

const authCookieName = "auth_token"

// Claims представляет JWT claims с информацией о пользователе.
type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID
}

// Auth middleware для аутентификации пользователей с помощью JWT.
func Auth(jwtSigningKey string) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var userID uuid.UUID

			cookie, err := r.Cookie(authCookieName)
			if err != nil {
				userID = uuid.New()
				cookie = &http.Cookie{
					HttpOnly: true,
					Name:     authCookieName,
				}

				if cookie.Value, err = createJWTString(jwtSigningKey, userID); err != nil {
					logger.Log.Errorw("Failed to create jwt token", "error", err)
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}

				http.SetCookie(w, cookie)
			} else {
				if userID, err = getUserID(jwtSigningKey, cookie.Value); err != nil {
					logger.Log.Errorw("Failed to get userID from auth token", "error", err)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}

			if userID == uuid.Nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			r.Header.Set("x-user-id", userID.String())
			h.ServeHTTP(w, r)
		})
	}
}

func createJWTString(jwtSigningKey string, userID uuid.UUID) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
	})

	tokenStr, err := token.SignedString([]byte(jwtSigningKey))
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func getUserID(jwtSigningKey, tokenStr string) (uuid.UUID, error) {
	if tokenStr == "" {
		return uuid.Nil, errors.New("token is empty")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.Errorf("unexpected signing method: %v", t.Header["alg"])
			}

			return []byte(jwtSigningKey), nil
		})
	if err != nil {
		return uuid.Nil, err
	}

	if !token.Valid {
		return uuid.Nil, errors.New("token is not valid")
	}

	return claims.UserID, nil
}
