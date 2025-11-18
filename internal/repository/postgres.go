package repository

import (
	"database/sql"

	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *Postgres {
	return &Postgres{db: db}
}

func (r *Postgres) PingDB() error {
	return r.db.Ping()
}

const getSQL = `SELECT id, original FROM shortener.urls WHERE id = $1`

func (r *Postgres) Get(id string) (*models.URL, error) {
	row := r.db.QueryRow(getSQL, id)

	var url models.URL
	err := row.Scan(&url.ID, &url.Original)
	return &url, err
}

const saveSQL = `INSERT INTO shortener.urls (id, original) VALUES ($1, $2)`

func (r *Postgres) Save(url *models.URL) error {
	_, err := r.db.Exec(saveSQL, url.ID, url.Original)
	return err
}

func (r *Postgres) Close() {
	if err := r.db.Close(); err != nil {
		logger.Log.Sugar().Errorw("failed to close db", "error", err)
	}
}
