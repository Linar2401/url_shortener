package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	conn *sql.DB
}

func New(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is empty")
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return d.conn.PingContext(ctx)
}

func (d *DB) Close() error {
	return d.conn.Close()
}
