package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// CivilizationHandler generates civilizations on demand. Admin-only —
// the endpoint is intended for internal testing and pregeneration
// tooling, not direct user traffic. No persistence in Phase 5.
type CivilizationHandler struct {
	// llm is the LLM backend. Nil if OPENAI_API_KEY was not set at
	// server start — the endpoint 503s rather than panicking so the
	// rest of the server stays bootable.
	llm llm.Client
}

// NewCivilizationHandler accepts a nil client so the server can boot
// without the API key; /generate returns 503 when client is nil.
func NewCivilizationHandler(client llm.Client) *CivilizationHandler {
	return &CivilizationHandler{llm: client}
}

// Generate rolls a fresh civilization through the five-step pipeline
// and returns it alongside the planet that seeded it. Admin/debug
// endpoint — no player_id, no persistence. Every call hits the LLM.
//
// @Summary      Generate a civilization
// @Description  Rolls a fresh civilization via the factory LLM pipeline. Returns the civ and the planet that seeded it. No persistence.
// @Tags         civilizations
// @Produce      json
// @Param        seed   query     int     false  "Seed override; defaults to time-based random"
// @Param        model  query     string  false  "LLM model override (e.g. gpt-4o)"
// @Success      200    {object}  map[string]interface{}  "{ civilization, planet }"
// @Failure      500    {string}  string  "generation error"
// @Failure      503    {string}  string  "OPENAI_API_KEY not configured"
// @Router       /api/civilizations/generate [post]
func (h *CivilizationHandler) Generate(w http.ResponseWriter, r *http.Request) {
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

	var opts []llm.Option
	if m := r.URL.Query().Get("model"); m != "" {
		opts = append(opts, llm.WithModel(m))
	}

	civ, planet, err := factory.GenerateCivilization(r.Context(), h.llm, seed, opts...)
	if err != nil {
		log.Printf("civgen seed=%d: %v", seed, err)
		http.Error(w, "generation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"civilization": civ,
		"planet":       planet,
	}); err != nil {
		log.Printf("encode civ response: %v", err)
	}
}
