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
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS players (
			id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			seed         INTEGER NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS ships (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_id  UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			loadout    JSONB NOT NULL DEFAULT '{}'::jsonb,
			state      JSONB NOT NULL DEFAULT '{}'::jsonb,
			transform  JSONB NOT NULL DEFAULT '{}'::jsonb,
			status     TEXT NOT NULL DEFAULT 'active'
				CHECK (status IN ('active','derelict','destroyed')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE players
			ADD COLUMN IF NOT EXISTS active_ship_id UUID
				REFERENCES ships(id) ON DELETE SET NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS one_active_ship_per_player
			ON ships(player_id) WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS ships_player_id_idx ON ships(player_id)`,
	}
	for _, s := range stmts {
		if _, err := database.ExecContext(context.Background(), s); err != nil {
			return fmt.Errorf("migration failed: %w\nstmt: %s", err, s)
		}
	}
	return nil
}
