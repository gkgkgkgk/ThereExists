package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type PlayerHandler struct {
	db *sql.DB
}

func NewPlayerHandler(db *sql.DB) *PlayerHandler {
	return &PlayerHandler{db: db}
}

type ShipResponse struct {
	ID        string          `json:"id"`
	Loadout   json.RawMessage `json:"loadout" swaggertype:"object"`
	State     json.RawMessage `json:"state" swaggertype:"object"`
	Transform json.RawMessage `json:"transform" swaggertype:"object"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}

type PlayerResponse struct {
	ID   string        `json:"id"`
	Seed int32         `json:"seed"`
	Ship *ShipResponse `json:"ship,omitempty"`
}

// GetPlayer godoc
// @Summary      Get or create a player
// @Description  Looks up an existing player by id, or creates a new player (with an active ship) if id is missing or not found.
// @Tags         player
// @Produce      json
// @Param        id   query     string  false  "Existing player UUID"
// @Success      200  {object}  PlayerResponse
// @Failure      500  {string}  string  "internal server error"
// @Router       /api/player [get]
func (h *PlayerHandler) GetPlayer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	playerID := r.URL.Query().Get("id")

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("begin tx: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var (
		seed         int32
		activeShipID sql.NullString
	)

	if playerID != "" {
		err := tx.QueryRowContext(ctx,
			`UPDATE players SET last_seen_at = NOW()
			 WHERE id = $1
			 RETURNING seed, active_ship_id`,
			playerID,
		).Scan(&seed, &activeShipID)

		if errors.Is(err, sql.ErrNoRows) {
			// Stale id from a wiped DB — fall through to create new
			playerID = ""
		} else if err != nil {
			log.Printf("lookup player %s: %v", playerID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if playerID == "" {
		playerID = uuid.New().String()
		seed = rand.New(rand.NewSource(time.Now().UnixNano())).Int31()

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO players (id, seed) VALUES ($1, $2)`,
			playerID, seed,
		); err != nil {
			log.Printf("insert player: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if !activeShipID.Valid {
		newShipID, err := createShipForPlayer(ctx, tx, playerID)
		if err != nil {
			log.Printf("create ship for %s: %v", playerID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		activeShipID = sql.NullString{String: newShipID, Valid: true}
	}

	ship, err := loadShip(ctx, tx, activeShipID.String)
	if err != nil {
		log.Printf("load ship %s: %v", activeShipID.String, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("commit: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PlayerResponse{
		ID:   playerID,
		Seed: seed,
		Ship: ship,
	})
}

func createShipForPlayer(ctx context.Context, tx *sql.Tx, playerID string) (string, error) {
	shipID := uuid.New().String()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO ships (id, player_id) VALUES ($1, $2)`,
		shipID, playerID,
	); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE players SET active_ship_id = $1 WHERE id = $2`,
		shipID, playerID,
	); err != nil {
		return "", err
	}
	return shipID, nil
}

func loadShip(ctx context.Context, tx *sql.Tx, shipID string) (*ShipResponse, error) {
	var s ShipResponse
	err := tx.QueryRowContext(ctx,
		`SELECT id, loadout, state, transform, status, created_at
		 FROM ships WHERE id = $1`,
		shipID,
	).Scan(&s.ID, &s.Loadout, &s.State, &s.Transform, &s.Status, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
