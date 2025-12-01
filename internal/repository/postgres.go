package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/models"
	"github.com/google/uuid"
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

func (r *Postgres) GetURL(id string) (models.URL, error) {
	row := r.db.QueryRow(getSQL, id)

	var url models.URL
	err := row.Scan(&url.ID, &url.Original)
	return url, err
}

const saveSQL = `
	INSERT INTO shortener.urls (id, original, creator_id)
	VALUES ($1, $2, $3)
	ON CONFLICT DO NOTHING
`

func (r *Postgres) Save(userID uuid.UUID, url models.URL) error {
	res, err := r.db.Exec(saveSQL, url.ID, url.Original, userID)
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

func (r *Postgres) SaveMany(userID uuid.UUID, urls []models.URL) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	for _, url := range urls {
		res, err := r.db.Exec(saveSQL, url.ID, url.Original, userID)
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

const getUserURLsSQL = `
	SELECT id, original
	FROM shortener.urls
	WHERE creator_id = $1
`

func (r *Postgres) GetUserURLs(userID uuid.UUID) ([]models.URL, error) {
	rows, err := r.db.Query(getUserURLsSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var urls []models.URL
	for rows.Next() {
		var url models.URL
		if err := rows.Scan(&url.ID, &url.Original); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		urls = append(urls, url)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error: %w", err)
	}

	return urls, nil
}

func (r *Postgres) Close() error {
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	return nil
}
