package handler

import (
	"context"
	"errors"

	pb "github.com/dsnikitin/shortener/api/proto" // путь к сгенерированным proto файлам
	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GRPCHandler представляет gRPC обработчики сервиса сокращения ссылок.
type GRPCHandler struct {
	pb.UnimplementedShortenerServiceServer
	shortURLBaseAddr string
	s                Service
	auditor          Auditor
}

// New создает новый экземпляр Handler.
func NewGRPCHandler(shortURLBaseAddr string, s Service, auditor Auditor) *GRPCHandler {
	return &GRPCHandler{
		shortURLBaseAddr: shortURLBaseAddr,
		s:                s,
		auditor:          auditor,
	}
}

// ShortenURL обрабатывает запрос на создание короткого URL.
// При успехе возвращает статус OK или AlreadyExists, если URL уже существует.
func (h *GRPCHandler) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty url")
	}

	id, err := h.s.CreateID(ctx, userID, req.GetUrl())
	if err != nil {
		var aeErr *errx.ErrAlreadyExists
		if !errors.As(err, &aeErr) {
			logger.Log.Errorw("Failed to create id", "error", err.Error())
			return nil, status.Error(codes.Internal, "internal server error")
		}

		// URL уже существует - возвращаем существующий ID с кодом AlreadyExists
		return pb.URLShortenResponse_builder{
			Result: h.shortURLBaseAddr + "/" + aeErr.URL.ID,
		}.Build(), status.Error(codes.AlreadyExists, "url already exists")
	}

	return pb.URLShortenResponse_builder{
		Result: h.shortURLBaseAddr + "/" + id,
	}.Build(), nil
}

// ExpandURL обрабатывает запрос по короткой ссылке и возвращает оригинальный URL.
// Если URL помечен как удаленный, возвращает статус FailedPrecondition (аналог HTTP 410).
func (h *GRPCHandler) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req.GetId() == "" || len(req.GetId()) > config.IDMaxLength {
		return nil, status.Error(codes.InvalidArgument, "incorrect id")
	}

	url, err := h.s.GetURL(ctx, req.GetId())
	if err != nil {
		if !errors.Is(err, errx.ErrNotFound) {
			logger.Log.Errorw("Failed to get original url", "error", err.Error())
			return nil, status.Error(codes.Internal, "internal server error")
		}
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if url.IsDeleted {
		return nil, status.Error(codes.FailedPrecondition, "url is deleted")
	}

	return pb.URLExpandResponse_builder{Result: url.Original}.Build(), nil
}

// ListUserURLs возвращает список всех неудаленных URL пользователя.
func (h *GRPCHandler) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	urls, err := h.s.GetUserURLs(ctx, userID)
	if err != nil {
		logger.Log.Errorw("Failed to get user urls", "userID", userID)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	resp := pb.UserURLsResponse_builder{
		Url: make([]*pb.URLData, 0, len(urls)),
	}

	for i := range urls {
		if !urls[i].IsDeleted {
			resp.Url = append(resp.Url, pb.URLData_builder{
				ShortUrl:    h.shortURLBaseAddr + "/" + urls[i].ID,
				OriginalUrl: urls[i].Original,
			}.Build())
		}
	}

	return resp.Build(), nil
}

// getUserID извлекает userID из контекста (должен быть добавлен интерцептором)
func getUserID(ctx context.Context) (uuid.UUID, error) {
	value := ctx.Value("userID")
	if value == nil {
		logger.Log.Errorw("User ID not found in context", "userID", value)
		return uuid.Nil, status.Error(codes.Internal, "internal server error")
	}

	userID, ok := value.(uuid.UUID)
	if !ok {
		logger.Log.Errorw("Invalid user ID type in context", "userID", value)
		return uuid.Nil, status.Error(codes.Internal, "internal server error")
	}

	return userID, nil
}
