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

// Migrate brings the schema to the current target shape. The pre-Phase-5.1
// players/ships/civilizations layout is dropped — generation is stateless
// (POST /api/ships/generate writes nothing) and the future start-game
// endpoint will write to the single `runs` table created here. Each row
// is one civ + ship + game-state bundle keyed by run id, with the player
// id attached and an `active` flag for the player's current run.
//
// Statements run in order; CASCADE on the drops handles any leftover FKs
// from older deployments. `runs` is reserved for the start-game endpoint
// — no code writes to it in this revision.
func Migrate(database *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS ships CASCADE`,
		`DROP TABLE IF EXISTS civilizations CASCADE`,
		`DROP TABLE IF EXISTS players CASCADE`,
		`CREATE TABLE IF NOT EXISTS runs (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_id       UUID NOT NULL,
			civilization    JSONB NOT NULL,
			ship            JSONB NOT NULL,
			state           JSONB NOT NULL DEFAULT '{}'::jsonb,
			active          BOOLEAN NOT NULL DEFAULT TRUE,
			factory_version TEXT NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS one_active_run_per_player
			ON runs(player_id) WHERE active`,
		`CREATE INDEX IF NOT EXISTS runs_player_id_idx ON runs(player_id)`,
	}
	for _, s := range stmts {
		if _, err := database.ExecContext(context.Background(), s); err != nil {
			return fmt.Errorf("migration failed: %w\nstmt: %s", err, s)
		}
	}
	return nil
}
