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

func (d *DB) SaveURL(code string, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//goland:noinspection SqlNoDataSourceInspection
	_, err := d.conn.ExecContext(ctx,
		"INSERT INTO urls (short_code, original_url) VALUES ($1, $2)",
		code, value,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %s", storage.ErrCollision, code)
		}
		return err
	}
	return nil
}

func (d *DB) SaveBatch(items []storage.BatchItem) error {
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
		"INSERT INTO urls (short_code, original_url) VALUES ($1, $2)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, item.ShortCode, item.OriginalURL); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: %s", storage.ErrCollision, item.ShortCode)
			}
			return err
		}
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

func (d *DB) Close() error {
	return d.conn.Close()
}
