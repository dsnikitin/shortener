package middleware

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/dsnikitin/shortener/internal/logger"
)

// AuthInterceptor для аутентификации пользователей с помощью JWT.
func AuthInterceptor(jwtSigningKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata is not provided")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "authorization metadata is empty")
		}

		userID, err := getUserID(jwtSigningKey, strings.TrimPrefix(values[0], "Bearer "))
		if err != nil {
			logger.Log.Errorw("Failed to get userID from auth token", "error", err)
			return nil, status.Error(codes.Unauthenticated, "invalid auth token")

		}

		ctx = context.WithValue(ctx, "userID", userID)
		return handler(ctx, req)
	}
}
