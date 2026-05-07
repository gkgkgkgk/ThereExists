package assembly

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// TestShipShape_SeedSweep — across a wide seed range, every generated
// ship has the FactoryVersion stamp, all three flight-slot keys
// present, and a non-null Short slot. A stray nil dereference or a
// slot key that silently disappears from the JSON breaks the frontend
// contract; catch it here instead of in the browser.
func TestShipShape_SeedSweep(t *testing.T) {
	const seeds = 200
	for seed := int64(0); seed < seeds; seed++ {
		l, err := GenerateRandomShip(seed, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		b, err := json.Marshal(l)
		if err != nil {
			t.Fatalf("seed %d: marshal: %v", seed, err)
		}
		var parsed struct {
			FactoryVersion string                     `json:"factory_version"`
			Flight         map[string]json.RawMessage `json:"flight"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("seed %d: unmarshal: %v", seed, err)
		}
		if parsed.FactoryVersion != FactoryVersion {
			t.Errorf("seed %d: factory_version = %q, want %q", seed, parsed.FactoryVersion, FactoryVersion)
		}
		for _, slot := range []string{"short", "medium", "far"} {
			if _, ok := parsed.Flight[slot]; !ok {
				t.Errorf("seed %d: flight.%s missing from JSON", seed, slot)
			}
		}
		if bytes.Equal(parsed.Flight["short"], []byte("null")) {
			t.Errorf("seed %d: flight.short is null", seed)
		}
	}
}

// TestFarUnlocksForTier5 — a tier-5 civ unlocks the Far slot (RBCA is
// gated to tier 5). The dispatcher reads tier directly from CivBias
// now, so the test just hands it a synthetic civ.
func TestFarUnlocksForTier5(t *testing.T) {
	tier5 := &factory.Civilization{
		ID:       "test-tier5",
		Name:     "Tier 5 Test",
		TechTier: 5,
	}
	for seed := int64(0); seed < 10; seed++ {
		l, err := GenerateRandomShip(seed, tier5)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if l.Flight[flight.Far] == nil {
			t.Fatalf("seed %d: flight.far nil under tier-5 civ — Far should populate", seed)
		}
	}
}
