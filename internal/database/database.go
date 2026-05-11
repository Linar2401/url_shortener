package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn *sql.DB
	dsn  string
}

func New(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is empty")
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &DB{conn: conn, dsn: dsn}, nil
}

func (d *DB) Migrate() error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, d.dsn)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("failed to init migrate: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

func (d *DB) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return d.conn.PingContext(ctx)
}

func (d *DB) SaveURL(code string, value string, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//goland:noinspection SqlNoDataSourceInspection
	_, err := d.conn.ExecContext(ctx,
		"INSERT INTO urls (short_code, original_url, user_id) VALUES ($1, $2, $3)",
		code, value, userID,
	)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}

	if pgErr.ConstraintName == "urls_original_url_key" {
		var existing string
		//goland:noinspection SqlNoDataSourceInspection
		qErr := d.conn.QueryRowContext(ctx,
			"SELECT short_code FROM urls WHERE original_url = $1",
			value,
		).Scan(&existing)
		if qErr != nil {
			return fmt.Errorf("failed to fetch existing short code: %w", qErr)
		}
		return &storage.ConflictError{ShortCode: existing}
	}

	return fmt.Errorf("%w: %s", storage.ErrCollision, code)
}

func (d *DB) SaveBatch(items []storage.BatchItem, userID string) error {
	if len(items) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	//goland:noinspection SqlNoDataSourceInspection
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO urls (short_code, original_url, user_id) VALUES ($1, $2, $3)
		 ON CONFLICT (original_url) DO UPDATE SET original_url = EXCLUDED.original_url
		 RETURNING short_code`,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for i, item := range items {
		var returnedCode string
		if err := stmt.QueryRowContext(ctx, item.ShortCode, item.OriginalURL, userID).Scan(&returnedCode); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: %s", storage.ErrCollision, item.ShortCode)
			}
			return err
		}
		items[i].ShortCode = returnedCode
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}
	return nil
}

func (d *DB) GetURL(code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var original string
	//goland:noinspection SqlNoDataSourceInspection
	err := d.conn.QueryRowContext(ctx,
		"SELECT original_url FROM urls WHERE short_code = $1",
		code,
	).Scan(&original)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("url not found: %s", code)
		}
		return "", err
	}
	return original, nil
}

func (d *DB) GetUserURLs(userID string) ([]storage.UserURL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//goland:noinspection SqlNoDataSourceInspection
	rows, err := d.conn.QueryContext(ctx,
		"SELECT short_code, original_url FROM urls WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []storage.UserURL
	for rows.Next() {
		var item storage.UserURL
		if err := rows.Scan(&item.ShortCode, &item.OriginalURL); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
