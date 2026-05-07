package flight_test

import (
	"math/rand"
	"testing"

	_ "github.com/gkgkgkgk/ThereExists/server/internal/factory/content"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// External test package; the blank import of factory/content registers
// the production archetype catalog so the dispatcher has Medium-slot
// entries to pick from.

// TestCivBias_ThrustVsIspShifts — drive the dispatcher with two civ
// profiles that prefer opposite ends of the thrust/Isp axis and assert
// the picked archetype mix differs in the expected direction. Loose
// because per-archetype weights and the rng add noise; we only check
// "did the punchy civ pick the punchy archetype more than the
// efficient civ did?" — a regression that drops the bias to zero would
// fail this test, but a small retune wouldn't.
func TestCivBias_ThrustVsIspShifts(t *testing.T) {
	punchy := &flight.CivBias{TechTier: 3, ThrustVsIspPreference: -1.0, RiskTolerance: 0.5}
	efficient := &flight.CivBias{TechTier: 3, ThrustVsIspPreference: 1.0, RiskTolerance: 0.5}

	punchyRDE := countMediumArchetype(t, punchy, "Rotating Detonation Manifold (RDE)", 1000)
	efficientRDE := countMediumArchetype(t, efficient, "Rotating Detonation Manifold (RDE)", 1000)
	if punchyRDE <= efficientRDE {
		t.Errorf("punchy civ picked RDE %d times, efficient civ %d — bias not shifting toward thrust",
			punchyRDE, efficientRDE)
	}

	punchySCTA := countMediumArchetype(t, punchy, "Staged Combustion Turbopump Assembly (SCTA)", 1000)
	efficientSCTA := countMediumArchetype(t, efficient, "Staged Combustion Turbopump Assembly (SCTA)", 1000)
	if efficientSCTA <= punchySCTA {
		t.Errorf("efficient civ picked SCTA %d times, punchy civ %d — bias not shifting toward Isp",
			efficientSCTA, punchySCTA)
	}
}

// TestCivBias_RiskTolerance_Sharpening — at risk=0 the weighting
// exponent is 2 (sharpens), so the workhorse Rarity=1.0 archetype
// should dominate harder than at risk=0.5 (no-op exponent).
func TestCivBias_RiskTolerance_Sharpening(t *testing.T) {
	conservative := &flight.CivBias{TechTier: 3, RiskTolerance: 0.0}
	wild := &flight.CivBias{TechTier: 3, RiskTolerance: 1.0}

	conservativeHPFA := countMediumArchetype(t, conservative, "Hypergolic Pressure-Fed Assembly (HPFA)", 1000)
	wildHPFA := countMediumArchetype(t, wild, "Hypergolic Pressure-Fed Assembly (HPFA)", 1000)
	if conservativeHPFA <= wildHPFA {
		t.Errorf("conservative civ picked HPFA %d times, wild civ %d — sharpening not boosting common archetype",
			conservativeHPFA, wildHPFA)
	}
}

// countMediumArchetype rolls n samples through GenerateForSlot for the
// Medium slot under the given bias and counts how many landed on the
// named archetype.
func countMediumArchetype(t *testing.T, civ *flight.CivBias, name string, n int) int {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	count := 0
	for i := 0; i < n; i++ {
		sys, err := flight.GenerateForSlot(flight.Medium, civ, rng)
		if err != nil {
			t.Fatalf("GenerateForSlot: %v", err)
		}
		if archetypeNameOf(sys) == name {
			count++
		}
	}
	return count
}

// archetypeNameOf inspects the embedded SystemBase via a type-assertion
// on the limited set of concrete flight types. Adding new flight
// categories means extending this switch.
func archetypeNameOf(sys flight.FlightSystem) string {
	switch v := sys.(type) {
	case *flight.LiquidChemicalEngine:
		return v.ArchetypeName
	case *flight.RelativisticDrive:
		return v.ArchetypeName
	}
	return ""
}
