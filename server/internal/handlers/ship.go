package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/assembly"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// ShipHandler now produces civ + ship bundles statelessly. The DB
// connection is retained on the struct so the future start-game
// endpoint (which writes to `runs`) can grow alongside this handler
// without rewiring main.go; nothing in /generate touches it.
type ShipHandler struct {
	db  *sql.DB
	llm llm.Client
}

func NewShipHandler(db *sql.DB, llmClient llm.Client) *ShipHandler {
	return &ShipHandler{db: db, llm: llmClient}
}

// GenerateResponse is the wire shape returned by POST /api/ships/generate.
// civilization, planet, and ship are emitted as the factory's native
// JSON. The caller is responsible for handing this to a future
// start-game endpoint if they want to persist a run.
type GenerateResponse struct {
	Seed         int64                  `json:"seed"`
	Civilization *factory.Civilization  `json:"civilization"`
	Planet       *factory.Planet        `json:"planet"`
	Ship         *assembly.ShipLoadout  `json:"ship"`
}

// Generate rolls a fresh civilization (LLM-driven) and a civ-aware ship
// in one shot, returning the bundle as JSON. No DB writes — persistence
// will be the job of the future start-game endpoint.
//
// @Summary      Generate a civilization + ship bundle
// @Description  Runs the civ pipeline (LLM, 3 round trips) then rolls a civ-aware ship via the factory. Returns both as JSON. No persistence — caller must POST to start-game (future endpoint) to durably create a run.
// @Tags         ships
// @Produce      json
// @Param        seed  query     int     false  "Seed override; defaults to time-based random. The same seed yields the same anchor planet but the LLM-driven civ steps remain non-deterministic."
// @Success      200   {object}  GenerateResponse
// @Failure      400   {string}  string  "invalid seed"
// @Failure      500   {string}  string  "generation error"
// @Failure      503   {string}  string  "OPENAI_API_KEY not configured"
// @Router       /api/ships/generate [post]
func (h *ShipHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if h.llm == nil {
		http.Error(w, "OPENAI_API_KEY not configured", http.StatusServiceUnavailable)
		return
	}

	seed := time.Now().UnixNano()
	if s := r.URL.Query().Get("seed"); s != "" {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "invalid seed", http.StatusBadRequest)
			return
		}
		seed = parsed
	}

	civ, planet, err := factory.GenerateCivilization(r.Context(), h.llm, seed)
	if err != nil {
		log.Printf("civgen seed=%d: %v", seed, err)
		http.Error(w, "generation error", http.StatusInternalServerError)
		return
	}

	loadout, err := assembly.GenerateRandomShip(seed, civ)
	if err != nil {
		log.Printf("ship gen seed=%d civ=%s: %v", seed, civ.ID, err)
		http.Error(w, "generation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(GenerateResponse{
		Seed:         seed,
		Civilization: civ,
		Planet:       planet,
		Ship:         loadout,
	}); err != nil {
		log.Printf("encode generate response: %v", err)
	}
}
