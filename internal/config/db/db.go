package db

import (
	"database/sql"

	"github.com/dsnikitin/shortener/internal/logger"
)

type Config struct {
	DSN string `env:"DATABASE_DSN"`
}

func NewPG(cfg *Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	logger.Log.Sugar().Infow("successfuly connected to database")
	return db, nil
}
