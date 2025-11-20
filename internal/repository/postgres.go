package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/dsnikitin/shortener/internal/errx"
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

const getSQL = `
	SELECT id, original
	FROM shortener.urls
	WHERE id = $1
`

func (r *Postgres) Get(id string) (models.URL, error) {
	row := r.db.QueryRow(getSQL, id)

	var url models.URL
	err := row.Scan(&url.ID, &url.Original)
	return url, err
}

const saveSQL = `
	INSERT INTO shortener.urls (id, original)
	VALUES ($1, $2)
	ON CONFLICT DO NOTHING
`

func (r *Postgres) Save(url models.URL) error {
	res, err := r.db.Exec(saveSQL, url.ID, url.Original)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	return nil
}

func (r *Postgres) SaveMany(urls []models.URL) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	for _, url := range urls {
		res, err := r.db.Exec(saveSQL, url.ID, url.Original)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("exec: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("rows affected: %w", err)
		}

		if rowsAffected == 0 {
			tx.Rollback()
			return errx.NewAlreadyExistsError(url, errors.New("already exists"))
		}
	}

	return tx.Commit()
}

func (r *Postgres) Close() {
	if err := r.db.Close(); err != nil {
		logger.Log.Sugar().Errorw("failed to close db", "error", err)
	}
}
