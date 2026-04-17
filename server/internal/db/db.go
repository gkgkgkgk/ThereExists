package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := database.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("db.Ping: %w", err)
	}
	return database, nil
}

func Migrate(database *sql.DB) error {
	_, err := database.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS players (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			seed       INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}
