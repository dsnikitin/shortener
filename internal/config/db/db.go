package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/dsnikitin/shortener/internal/logger"
)

type Config struct {
	DSN string `env:"DATABASE_DSN"`
}

func New(cfg *Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}

	logger.Log.Sugar().Infow("successfuly connected to database")
	return db, nil
}
