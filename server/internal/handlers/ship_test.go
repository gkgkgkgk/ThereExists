package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/gkgkgkgk/ThereExists/server/internal/db"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// seedCivAndLink inserts a synthetic civilizations row and links it to
// the player via civ_id. Used by tests that need to exercise the
// handler's "civ already exists, just load it" path without spinning up
// an LLM. The civ uses GenericCivilization-shaped TechProfile
// (no preferences) so the rolled ship matches the legacy civ-blind
// expectations.
func seedCivAndLink(t *testing.T, database *sql.DB, playerID string) {
	t.Helper()
	civID := uuid.NewString()
	planet, err := factory.GeneratePlanet(7)
	if err != nil {
		t.Fatalf("GeneratePlanet: %v", err)
	}
	civ := &factory.Civilization{
		ID:                   civID,
		Name:                 "Test Civ",
		Description:          "Synthetic test civilization.",
		HomeworldDescription: "Synthetic homeworld.",
		AgeYears:             1000,
		TechTier:             3,
		Flavor:               "test",
	}
	if err := factory.SaveCivilization(context.Background(), database, civ, planet, "test"); err != nil {
		t.Fatalf("SaveCivilization: %v", err)
	}
	if _, err := database.Exec(`UPDATE players SET civ_id = $1 WHERE id = $2`, civID, playerID); err != nil {
		t.Fatalf("link civ: %v", err)
	}
}

// Phase 3 introduces the handler integration pattern. The test hits a
// real Postgres (TEST_DATABASE_URL), exercises POST /api/ships/generate
// end-to-end, and asserts the loadout round-trips. Skipped when the env
// var is absent — don't block Phase 3 on CI infra that isn't in place.

func requireTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping handler integration test")
	}
	database, err := db.Connect(url)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	return database
}

func TestShipHandler_Generate_RoundTrip(t *testing.T) {
	database := requireTestDB(t)

	// Picker + civ tier wiring — production does this in main.go.
	flight.SetManufacturerPicker(factory.PickManufacturer)
	flight.SetCivTechTierLookup(func(id string) (int, bool) {
		c, ok := factory.Civilizations[id]
		if !ok {
			return 0, false
		}
		return c.TechTier, true
	})

	// Fresh player + ship for this test. Use the real PlayerHandler so
	// the setup matches production creation flow.
	ph := NewPlayerHandler(database)
	playerReq := httptest.NewRequest("GET", "/api/player", nil)
	playerResp := httptest.NewRecorder()
	ph.GetPlayer(playerResp, playerReq)
	if playerResp.Code != http.StatusOK {
		t.Fatalf("player create: status %d: %s", playerResp.Code, playerResp.Body.String())
	}
	var player PlayerResponse
	if err := json.Unmarshal(playerResp.Body.Bytes(), &player); err != nil {
		t.Fatalf("decode player: %v", err)
	}

	// Phase 5.1 lazily provisions a civ on the first /generate call —
	// requires an LLM client. The integration test runs without one, so
	// pre-seed a civilizations row and link it to the player. The
	// handler's resolveCiv path then loads it instead of generating.
	seedCivAndLink(t, database, player.ID)

	// Hit /api/ships/generate.
	sh := NewShipHandler(database, nil)
	genReq := httptest.NewRequest("POST", "/api/ships/generate?player_id="+player.ID, nil)
	genResp := httptest.NewRecorder()
	sh.Generate(genResp, genReq)
	if genResp.Code != http.StatusOK {
		t.Fatalf("generate: status %d: %s", genResp.Code, genResp.Body.String())
	}

	// Response body: parseable JSON with a populated short slot.
	var body struct {
		FactoryVersion string                     `json:"factory_version"`
		Flight         map[string]json.RawMessage `json:"flight"`
	}
	if err := json.Unmarshal(genResp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode generate body: %v", err)
	}
	if body.FactoryVersion == "" {
		t.Error("factory_version empty in response")
	}
	if string(body.Flight["short"]) == "null" {
		t.Error("flight.short is null in response — Phase 3 must populate it")
	}

	// DB row: loadout is persisted JSONB with a non-null short slot.
	var shortIsNotNull bool
	var factoryVersion sql.NullString
	if err := database.QueryRow(
		`SELECT (loadout -> 'flight' -> 'short') IS NOT NULL, factory_version
		   FROM ships WHERE player_id = $1 AND status = 'active'`,
		player.ID,
	).Scan(&shortIsNotNull, &factoryVersion); err != nil {
		t.Fatalf("read back ship: %v", err)
	}
	if !shortIsNotNull {
		t.Error("ships.loadout->'flight'->'short' was NULL after /generate")
	}
	if !factoryVersion.Valid || factoryVersion.String == "" {
		t.Error("ships.factory_version not persisted")
	}

	// GET /api/player round-trips the same ship_id.
	roundtripReq := httptest.NewRequest("GET", "/api/player?id="+player.ID, nil)
	roundtripResp := httptest.NewRecorder()
	ph.GetPlayer(roundtripResp, roundtripReq)
	if roundtripResp.Code != http.StatusOK {
		t.Fatalf("player roundtrip: status %d", roundtripResp.Code)
	}
	var roundtrip PlayerResponse
	if err := json.Unmarshal(roundtripResp.Body.Bytes(), &roundtrip); err != nil {
		t.Fatalf("decode roundtrip: %v", err)
	}
	if player.Ship == nil || roundtrip.Ship == nil {
		t.Fatalf("ship missing: player=%v roundtrip=%v", player.Ship, roundtrip.Ship)
	}
	if roundtrip.Ship.ID != player.Ship.ID {
		t.Errorf("ship id changed across calls: %s -> %s", player.Ship.ID, roundtrip.Ship.ID)
	}
}
