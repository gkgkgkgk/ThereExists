package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/assembly"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

type ShipHandler struct {
	db  *sql.DB
	llm llm.Client // nil-safe: the handler 503s when civ provisioning is needed but no client exists.
}

// NewShipHandler accepts a nil LLM client so the server boots without
// OPENAI_API_KEY. /generate then 503s only when the player has no civ
// yet (i.e. provisioning is genuinely required).
func NewShipHandler(db *sql.DB, llmClient llm.Client) *ShipHandler {
	return &ShipHandler{db: db, llm: llmClient}
}

// Generate rolls a ship via the factory and persists the result onto the
// caller's active ships row. Manual-only debug endpoint — not wired into
// the UI in Phase 3. Determinism: the seed defaults to the player's
// seed, so repeated calls return the same ship. Pass ?seed=<int> to
// override (useful for inspecting variety without creating new players).
//
// Phase 5.1: the first call for a civ-less player generates and persists
// a Civilization, links it to the player, and rolls a civ-aware ship.
// Subsequent calls load the existing civ (no LLM round trip).
//
// @Summary      Generate a ship loadout
// @Description  Rolls a ship via the factory and persists it onto the player's active ship. Defaults to the player's seed; pass ?seed= to override. First call for a civ-less player provisions a civilization (LLM-driven) and links it.
// @Tags         ships
// @Produce      json
// @Param        player_id  query     string  true   "Player UUID"
// @Param        seed       query     int     false  "Optional seed override"
// @Success      200        {object}  map[string]interface{}  "Ship loadout JSON"
// @Failure      400        {string}  string  "missing player_id query param"
// @Failure      404        {string}  string  "no active ship for player"
// @Failure      500        {string}  string  "internal server error"
// @Failure      503        {string}  string  "OPENAI_API_KEY not configured (civ provisioning required)"
// @Router       /api/ships/generate [post]
func (h *ShipHandler) Generate(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "missing player_id query param", http.StatusBadRequest)
		return
	}

	var (
		shipID    string
		civID     sql.NullString
		baseSeed  int32
	)
	// Single read pulls everything we need for routing — ship row, civ_id,
	// and the player's persistent seed (used for both seed-resolution and
	// civ-generation seeding).
	err := h.db.QueryRowContext(r.Context(),
		`SELECT s.id, p.civ_id, p.seed
		   FROM ships s JOIN players p ON p.id = s.player_id
		  WHERE s.player_id = $1 AND s.status = 'active'
		  LIMIT 1`,
		playerID,
	).Scan(&shipID, &civID, &baseSeed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no active ship for player", http.StatusNotFound)
			return
		}
		log.Printf("ship lookup for player %s: %v", playerID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	civ, err := h.resolveCiv(r.Context(), playerID, civID, int64(baseSeed))
	if err != nil {
		if errors.Is(err, errLLMUnavailable) {
			http.Error(w, "OPENAI_API_KEY not configured", http.StatusServiceUnavailable)
			return
		}
		log.Printf("resolve civ for player %s: %v", playerID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	rolloutSeed, err := h.resolveSeed(r, baseSeed)
	if err != nil {
		log.Printf("resolve seed for player %s: %v", playerID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	loadout, err := assembly.GenerateRandomShip(rolloutSeed, civ)
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
		`UPDATE ships SET loadout = $1::jsonb, factory_version = $2, civ_id = $3 WHERE id = $4`,
		string(loadoutJSON), assembly.FactoryVersion, civ.ID, shipID,
	); err != nil {
		log.Printf("persist loadout for ship %s: %v", shipID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(loadoutJSON)
}

// errLLMUnavailable signals that civ provisioning is required but the
// handler has no LLM client. Wrapped for the caller so they can map it
// to a 503 without string-matching.
var errLLMUnavailable = errors.New("ship handler: civ provisioning required but no LLM client configured")

// resolveCiv loads an existing civilization for the player or, on first
// call, generates one and links it. Returns the *Civilization the ship
// roll should consume. The civ-generation seed is the player's
// persistent seed so the same player always anchors on the same planet,
// even if the LLM steps drift on retry.
func (h *ShipHandler) resolveCiv(ctx context.Context, playerID string, civID sql.NullString, seed int64) (*factory.Civilization, error) {
	if civID.Valid {
		civ, _, err := factory.LoadCivilization(ctx, h.db, civID.String)
		if err != nil {
			return nil, err
		}
		return civ, nil
	}

	if h.llm == nil {
		return nil, errLLMUnavailable
	}

	civ, planet, err := factory.GenerateCivilization(ctx, h.llm, seed)
	if err != nil {
		return nil, err
	}

	// Wrap the SaveCivilization + UPDATE players in a transaction so a
	// crash between the two doesn't leave an orphan civilizations row
	// dangling. Both writes are small; serial-isolation default is fine.
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }() // no-op if Commit already ran

	// SaveCivilization takes *sql.DB — but inside a tx we need the tx's
	// connection. Reuse the same write logic by inlining; the helper
	// stays useful for non-transactional callers (e.g. admin endpoints).
	if err := saveCivilizationTx(ctx, tx, civ, planet, assembly.FactoryVersion); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE players SET civ_id = $1 WHERE id = $2`,
		civ.ID, playerID,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return civ, nil
}

// saveCivilizationTx mirrors factory.SaveCivilization but runs inside an
// open *sql.Tx. Inlined here rather than exported on the factory side
// because the tx-vs-DB split is a handler concern; factory's public
// surface stays sql.DB-shaped.
func saveCivilizationTx(ctx context.Context, tx *sql.Tx, civ *factory.Civilization, planet *factory.Planet, factoryVersion string) error {
	profileJSON, err := json.Marshal(civ.TechProfile)
	if err != nil {
		return err
	}
	planetJSON, err := json.Marshal(planet)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO civilizations
			(id, name, description, homeworld_desc, age_years, tech_tier, flavor, profile, planet, factory_version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		civ.ID, civ.Name, civ.Description, civ.HomeworldDescription,
		civ.AgeYears, civ.TechTier, civ.Flavor, profileJSON, planetJSON, factoryVersion,
	)
	return err
}

// resolveSeed returns the seed to use for ship-roll generation: ?seed
// override if present and parseable, else the player's persistent seed.
// Note: the civ's anchor planet always uses the player's seed (not the
// override), so ?seed lets you inspect different ships under the same
// civ.
func (h *ShipHandler) resolveSeed(r *http.Request, base int32) (int64, error) {
	if s := r.URL.Query().Get("seed"); s != "" {
		return strconv.ParseInt(s, 10, 64)
	}
	return int64(base), nil
}
