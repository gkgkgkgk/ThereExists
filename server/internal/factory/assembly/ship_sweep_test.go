package assembly

import (
	"bytes"
	"encoding/json"
	"strings"
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

// TestFarUnlocksForTier5 — when the civ tier lookup reports tier 5,
// Far populates (RBCA is the only Phase 4 entry and is gated to tier
// 5). Swaps the injected lookup rather than authoring a tier-5 civ in
// the registry, so this test doesn't creep into content territory.
func TestFarUnlocksForTier5(t *testing.T) {
	flight.SetCivTechTierLookup(func(id string) (int, bool) { return 5, true })
	t.Cleanup(func() {
		// Restore the production wiring so later tests in this binary
		// see the real civ-tier map.
		flight.SetCivTechTierLookup(func(id string) (int, bool) {
			c, ok := factory.Civilizations[id]
			if !ok {
				return 0, false
			}
			return c.TechTier, true
		})
	})

	// Scan a handful of seeds — RBCA is the only tier-5 archetype in
	// Far, so every seed should populate it once the gate opens.
	for seed := int64(0); seed < 10; seed++ {
		l, err := GenerateRandomShip(seed, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if l.Flight[flight.Far] == nil {
			t.Fatalf("seed %d: flight.far nil under tier-5 lookup — Far should populate", seed)
		}
	}
}

// TestSameManufacturerBias — over many seeds, the ship-level
// provenance bias should make Short and Medium share a manufacturer
// more often than the ~1/3 chance a blind uniform pick would yield on
// the three-manufacturer generic roster. We don't pin a tight ratio
// (sampling noise on ~200 seeds + weight-zero filtering for
// archetype-specific exclusions), just assert "meaningfully above
// uniform" so a future regression that drops the bias shows up.
func TestSameManufacturerBias(t *testing.T) {
	const seeds = 300
	matched, total := 0, 0
	for seed := int64(0); seed < seeds; seed++ {
		l, err := GenerateRandomShip(seed, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		shortMfg := extractManufacturerID(t, l.Flight[flight.Short])
		mediumMfg := extractManufacturerID(t, l.Flight[flight.Medium])
		if shortMfg == "" || mediumMfg == "" {
			continue
		}
		total++
		if shortMfg == mediumMfg {
			matched++
		}
	}
	if total == 0 {
		t.Fatal("no seed produced both Short and Medium — sweep setup is broken")
	}
	ratio := float64(matched) / float64(total)
	// Uniform baseline is ~1/3; bias=3× on equal weights targets ~60%.
	// 0.45 is the loose floor that catches "bias dropped to 0".
	if ratio < 0.45 {
		t.Errorf("same-manufacturer rate %.2f (%d/%d) below floor 0.45 — provenance bias may be broken",
			ratio, matched, total)
	}
}

// extractManufacturerID round-trips the flight system through JSON to
// read the serialised manufacturer_id without importing each concrete
// type. Returns "" if the slot is nil.
func extractManufacturerID(t *testing.T, sys any) string {
	t.Helper()
	if sys == nil {
		return ""
	}
	b, err := json.Marshal(sys)
	if err != nil {
		t.Fatalf("marshal system: %v", err)
	}
	var m struct {
		ManufacturerID string `json:"manufacturer_id"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		// Not all future system types may have this field in the same
		// place; surface as a skip signal rather than failing.
		if strings.Contains(err.Error(), "manufacturer_id") {
			return ""
		}
		t.Fatalf("unmarshal system: %v", err)
	}
	return m.ManufacturerID
}
