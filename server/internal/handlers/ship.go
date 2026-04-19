package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory/assembly"
)

type ShipHandler struct {
	db *sql.DB
}

func NewShipHandler(db *sql.DB) *ShipHandler {
	return &ShipHandler{db: db}
}

// Generate rolls a ship via the factory and persists the result onto the
// caller's active ships row. Manual-only debug endpoint — not wired into
// the UI in Phase 3. Determinism: the seed defaults to the player's
// seed, so repeated calls return the same ship. Pass ?seed=<int> to
// override (useful for inspecting variety without creating new players).
func (h *ShipHandler) Generate(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "missing player_id query param", http.StatusBadRequest)
		return
	}

	var shipID string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT id FROM ships WHERE player_id = $1 AND status = 'active' LIMIT 1`,
		playerID,
	).Scan(&shipID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no active ship for player", http.StatusNotFound)
			return
		}
		log.Printf("ship lookup for player %s: %v", playerID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	seed, err := h.resolveSeed(r, playerID)
	if err != nil {
		log.Printf("resolve seed for player %s: %v", playerID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	loadout, err := assembly.GenerateRandomShip(seed)
	if err != nil {
		log.Printf("generate ship for player %s: %v", playerID, err)
		http.Error(w, "factory error", http.StatusInternalServerError)
		return
	}

	loadoutJSON, err := json.Marshal(loadout)
	if err != nil {
		log.Printf("marshal loadout: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if _, err := h.db.ExecContext(r.Context(),
		`UPDATE ships SET loadout = $1::jsonb, factory_version = $2 WHERE id = $3`,
		string(loadoutJSON), assembly.FactoryVersion, shipID,
	); err != nil {
		log.Printf("persist loadout for ship %s: %v", shipID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(loadoutJSON)
}

// resolveSeed returns the seed to use for generation: ?seed override if
// present and parseable, else the player's persistent seed.
func (h *ShipHandler) resolveSeed(r *http.Request, playerID string) (int64, error) {
	if s := r.URL.Query().Get("seed"); s != "" {
		return strconv.ParseInt(s, 10, 64)
	}
	var seed int32
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT seed FROM players WHERE id = $1`,
		playerID,
	).Scan(&seed); err != nil {
		return 0, err
	}
	return int64(seed), nil
}
