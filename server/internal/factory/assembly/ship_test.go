package assembly

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// TestMain wires the flight dispatcher to the factory's manufacturer
// picker — main.go does this in production, but tests don't run main.
func TestMain(m *testing.M) {
	flight.SetManufacturerPicker(factory.PickManufacturer)
	os.Exit(m.Run())
}

// TestGenerateRandomShip_Determinism — same seed → bit-equal JSON.
// UUIDs make struct-equality fragile (every engine gets a fresh ID), so
// compare the full marshaled output and assert the two JSONs match
// after stripping the per-instance "id" fields.
func TestGenerateRandomShip_Determinism(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		a, err := GenerateRandomShip(seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		b, err := GenerateRandomShip(seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		aJSON, _ := json.Marshal(a)
		bJSON, _ := json.Marshal(b)
		aStripped := stripIDs(t, aJSON)
		bStripped := stripIDs(t, bJSON)
		if !bytes.Equal(aStripped, bStripped) {
			t.Fatalf("seed %d: non-deterministic output\n  a=%s\n  b=%s", seed, aStripped, bStripped)
		}
	}
}

// TestGenerateRandomShip_JSONShape — Medium and Far serialise as
// explicit JSON null so the frontend contract stays stable.
func TestGenerateRandomShip_JSONShape(t *testing.T) {
	l, err := GenerateRandomShip(42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		FactoryVersion string          `json:"factory_version"`
		Flight         map[string]json.RawMessage `json:"flight"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.FactoryVersion != FactoryVersion {
		t.Errorf("factory_version = %q, want %q", parsed.FactoryVersion, FactoryVersion)
	}
	for _, slot := range []string{"short", "medium", "far"} {
		if _, ok := parsed.Flight[slot]; !ok {
			t.Errorf("flight slot %q missing from JSON (must be present even when null)", slot)
		}
	}
	if bytes.Equal(parsed.Flight["short"], []byte("null")) {
		t.Error("flight.short is null — Short must populate on every ship")
	}
	if bytes.Equal(parsed.Flight["medium"], []byte("null")) {
		t.Error("flight.medium is null — Phase 4 registers Medium archetypes for all civs")
	}
	// Far: RBCA is registered (commit 5) but TechTier gating lands in
	// commit 6. Until then every ship rolls Far regardless of civ tier.
	// commit 6 tightens this assertion back to expecting null for tier-3
	// civs.
	if _, ok := parsed.Flight["far"]; !ok {
		t.Error("flight.far missing from JSON")
	}
}

// stripIDs removes every "id": "<uuid>" field from a JSON blob so two
// runs of the same seed can be byte-compared. Operates on the parsed
// form to avoid regex gymnastics.
func stripIDs(t *testing.T, b []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stripIDsRecursive(v)
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

func stripIDsRecursive(v any) {
	switch x := v.(type) {
	case map[string]any:
		delete(x, "id")
		for _, child := range x {
			stripIDsRecursive(child)
		}
	case []any:
		for _, child := range x {
			stripIDsRecursive(child)
		}
	}
}
