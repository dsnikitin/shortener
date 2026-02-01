package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsnikitin/shortener/internal/errx"
	"github.com/dsnikitin/shortener/internal/logger"
	"github.com/dsnikitin/shortener/internal/models"
)

// Postgres представляет PostgreSQL хранилище для URL.
type Postgres struct {
	db *pgxpool.Pool
}

// NewPostgres создает новое PostgreSQL хранилище.
func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

// PingDB проверяет соединение с базой данных PostgreSQL.
func (r *Postgres) PingDB(ctx context.Context) error {
	return r.db.Ping(ctx)
}

const getSQL = `
	SELECT id, original, creator_id, is_deleted
	FROM shortener.urls
	WHERE id = @id
`

// GetURL возвращает URL по его короткой ссылке.
func (r *Postgres) GetURL(ctx context.Context, id string) (models.URL, error) {
	row := r.db.QueryRow(ctx, getSQL, pgx.NamedArgs{"id": id})

	var url models.URL
	if err := row.Scan(&url.ID, &url.Original, &url.CreatorID, &url.IsDeleted); err != nil {
		return models.URL{}, fmt.Errorf("scan: %w", err)
	}

	return url, nil
}

const saveSQL = `
	INSERT INTO shortener.urls (id, original, creator_id)
	VALUES (@id, @original, @creatorID)
	ON CONFLICT DO NOTHING
`

// Save сохраняет URL в PostgreSQL.
func (r *Postgres) Save(ctx context.Context, url models.URL) error {
	res, err := r.db.Exec(
		ctx, saveSQL, pgx.NamedArgs{"id": url.ID, "original": url.Original, "creatorID": url.CreatorID})
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	if res.RowsAffected() == 0 {
		return errx.NewAlreadyExistsError(url, errors.New("already exists"))
	}

	return nil
}

// SaveMany сохраняет несколько URLs в PostgreSQL.
func (r *Postgres) SaveMany(ctx context.Context, urls []models.URL) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for i := range urls {
		batch.Queue(saveSQL,
			pgx.NamedArgs{
				"id":        urls[i].ID,
				"original":  urls[i].Original,
				"creatorID": urls[i].CreatorID,
			},
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := range urls {
		res, err := br.Exec()
		if err != nil {
			return fmt.Errorf("exec error on url %s: %w", urls[i].Original, err)
		}

		if res.RowsAffected() == 0 {
			return errx.NewAlreadyExistsError(urls[i], errors.New("already exists"))
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

const getUserURLsSQL = `
	SELECT id, original, creator_id, is_deleted
	FROM shortener.urls
	WHERE creator_id = @creatorID
`

// GetUserURLs возвращает все URLs пользователя из PostgreSQL.
func (r *Postgres) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]models.URL, error) {
	rows, err := r.db.Query(ctx, getUserURLsSQL, pgx.NamedArgs{"creatorID": userID})
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var urls []models.URL
	for rows.Next() {
		var url models.URL
		if err := rows.Scan(&url.ID, &url.Original, &url.CreatorID, &url.IsDeleted); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		urls = append(urls, url)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error: %w", err)
	}

	return urls, nil
}

const deleteUserURLsSQL = `
	UPDATE shortener.urls
	SET is_deleted = true
	WHERE id = @id AND creator_id = @creatorID
`

// DeleteURLs помечает URLs как удаленные в PostgreSQL.
func (r *Postgres) DeleteURLs(ctx context.Context, urls []models.DeletableURL) {
	batch := &pgx.Batch{}
	for i := range urls {
		batch.Queue(deleteUserURLsSQL,
			pgx.NamedArgs{
				"id":        urls[i].ID,
				"creatorID": urls[i].CreatorID,
			},
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for i := range urls {
		_, err := br.Exec()
		if err != nil {
			logger.Log.Errorw("Failed to delete url", "ID", urls[i].ID, "error", err)
		}
	}
}

// Close закрывает соединение с PostgreSQL.
func (r *Postgres) Close() {
	r.db.Close()
}
