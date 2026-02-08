package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsnikitin/shortener/internal/logger"
)

// Config содержит конфигурацию подключения к базе данных.
type Config struct {
	DSN string `env:"DSN"`
}

// New создает пул соединений с базой данных PostgreSQL.
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

	logger.Log.Infow("Successfully connected to database")
	return pool, nil
}
