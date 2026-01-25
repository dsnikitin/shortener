package db

import (
	"context"
	"fmt"
	"time"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DSN string `env:"DATABASE_DSN"`
}

func New(cfg *Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Log.Sugar().Infow("successfuly connected to database")
	return pool, nil
}
