package handlers

import (
	"database/sql"
	"encoding/json"
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

type PlayerResponse struct {
	ID   string `json:"id"`
	Seed int32  `json:"seed"`
}

func (h *PlayerHandler) GetPlayer(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("id")

	var seed int32

	if playerID != "" {
		// Returning player — update last_seen_at and return their seed
		err := h.db.QueryRowContext(r.Context(),
			`UPDATE players SET last_seen_at = NOW() WHERE id = $1 RETURNING seed`,
			playerID,
		).Scan(&seed)

		if err == nil {
			// Found — return existing player
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(PlayerResponse{ID: playerID, Seed: seed})
			return
		}
		if err != sql.ErrNoRows {
			log.Printf("error looking up player %s: %v", playerID, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// Player ID not found — fall through to create new
	}

	// New player
	newID := uuid.New().String()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seed = rng.Int31()

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO players (id, seed) VALUES ($1, $2)`,
		newID, seed,
	)
	if err != nil {
		log.Printf("error creating player: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PlayerResponse{ID: newID, Seed: seed})
}
