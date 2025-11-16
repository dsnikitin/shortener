package repository

import (
	"database/sql"
	"errors"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (r *PostgresStorage) PingDB() error {
	return r.db.Ping()
}

func (r *PostgresStorage) Get(id string) (*models.URL, error) {
	return nil, errors.New("not implemented")
}

func (r *PostgresStorage) Save(url *models.URL) error {
	return errors.New("not implemented")
}

func (r *PostgresStorage) Close() {
	if err := r.db.Close(); err != nil {
		logger.Log.Sugar().Errorw("failed to close db", "error", err)
	}
}
