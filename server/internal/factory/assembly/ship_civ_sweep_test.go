package assembly

import (
	"encoding/json"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// TestCivDivergence_SweepsDiffer — synthesise a conservative and a wild
// civ (no DB, no LLM) and roll 200 ships each. The conservative civ
// should pick the workhorse Medium archetype (HPFA) more often; the
// wild civ should pick rare archetypes (RDE, SABRE, SCTA) more often,
// and its rolled health should land lower on average. This is the
// canary that the dial multipliers in commits 5–7 actually do
// something — if it ever passes with bit-equal stats, the dials are
// silently no-ops.
func TestCivDivergence_SweepsDiffer(t *testing.T) {
	conservative := &factory.Civilization{
		ID:       "test-conservative",
		Name:     "Conservative",
		TechTier: 3,
		TechProfile: factory.TechProfile{
			RiskTolerance:         0.05,
			ThrustVsIspPreference: -0.5,
			PreferredMixtureIDs:   []string{"MMH_NTO"},
			AversionToCryogenics:  0.0,
		},
	}
	wild := &factory.Civilization{
		ID:       "test-wild",
		Name:     "Wild",
		TechTier: 3,
		TechProfile: factory.TechProfile{
			RiskTolerance:         0.95,
			ThrustVsIspPreference: 0.7,
			PreferredMixtureIDs:   []string{"Methalox"},
			AversionToCryogenics:  0.0,
		},
	}

	// Register the synthetic civs + a single placeholder manufacturer
	// each so the dispatcher's CivTechTierLookup and the manufacturer
	// picker can resolve them. Phase 5.1 keeps the manufacturer pool
	// shared (manufacturer-from-civ is out of scope), but the picker
	// filters by CivilizationID — so the test attaches one mfg per
	// synthetic civ rather than altering production wiring.
	registerSyntheticCiv(t, conservative)
	registerSyntheticCiv(t, wild)

	consStats := sweep(t, conservative, 200)
	wildStats := sweep(t, wild, 200)

	// HPFA (Rarity 1.0, the workhorse) should dominate the conservative
	// civ's Medium picks far more than the wild civ's. Loose floor.
	if consStats.mediumArchetypeCounts["Hypergolic Pressure-Fed Assembly (HPFA)"] <=
		wildStats.mediumArchetypeCounts["Hypergolic Pressure-Fed Assembly (HPFA)"] {
		t.Errorf("HPFA: conservative=%d wild=%d — risk-sharpening toward workhorse not visible",
			consStats.mediumArchetypeCounts["Hypergolic Pressure-Fed Assembly (HPFA)"],
			wildStats.mediumArchetypeCounts["Hypergolic Pressure-Fed Assembly (HPFA)"])
	}

	// Rare archetypes (RDE + SABRE + SCTA) combined: wild should land
	// at least 2× the conservative count. Loose because sample noise.
	consRare := consStats.mediumArchetypeCounts["Rotating Detonation Manifold (RDE)"] +
		consStats.mediumArchetypeCounts["Synthetically Actuated Biogenic Reaction Engine (SABRE)"] +
		consStats.mediumArchetypeCounts["Staged Combustion Turbopump Assembly (SCTA)"]
	wildRare := wildStats.mediumArchetypeCounts["Rotating Detonation Manifold (RDE)"] +
		wildStats.mediumArchetypeCounts["Synthetically Actuated Biogenic Reaction Engine (SABRE)"] +
		wildStats.mediumArchetypeCounts["Staged Combustion Turbopump Assembly (SCTA)"]
	if wildRare < 2*consRare {
		t.Errorf("rare archetype counts: wild=%d conservative=%d — risk flattening not boosting rarities",
			wildRare, consRare)
	}

	// Mean health: conservative civ rolls higher than wild — risk shifts
	// effectiveLo upward. Margin floor is loose; the rollHealth shift is
	// strong enough that this gap should easily clear 0.05.
	if consStats.meanHealth <= wildStats.meanHealth+0.05 {
		t.Errorf("mean health: conservative=%.3f wild=%.3f — RiskTolerance shift on HealthInitRange not visible",
			consStats.meanHealth, wildStats.meanHealth)
	}
}

// TestNoCivBackwardCompat — GenerateRandomShip(seed, nil) over a wide
// seed range still respects the existing sweep invariants. Belt-and-
// braces against the civ-aware path leaking into the no-civ path.
func TestNoCivBackwardCompat(t *testing.T) {
	const seeds = 200
	for seed := int64(0); seed < seeds; seed++ {
		l, err := GenerateRandomShip(seed, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if l.Flight[flight.Short] == nil {
			t.Errorf("seed %d: flight.short nil — Short must populate on every seed", seed)
		}
		// GenericCivilization is tier 3; RBCA is tier 5. Far stays nil.
		if l.Flight[flight.Far] != nil {
			t.Errorf("seed %d: flight.far populated under tier-3 generic civ — gating broken", seed)
		}
	}
}

// TestFactoryVersionPhase5_1 — trivial; locks the version stamp.
func TestFactoryVersionPhase5_1(t *testing.T) {
	if FactoryVersion != "phase5_1-v1" {
		t.Errorf("FactoryVersion = %q, want %q", FactoryVersion, "phase5_1-v1")
	}
}

// registerSyntheticCiv is a no-op now — the dispatcher reads tier and
// preferences directly from the *factory.Civilization passed into
// GenerateRandomShip, so no global registration is needed. Kept as a
// named hook so adding registry-touching setup back here later (if
// content registration grows test fixtures) is a one-line change.
func registerSyntheticCiv(t *testing.T, civ *factory.Civilization) {
	t.Helper()
	_ = civ
}

// civSweepStats accumulates the metrics the divergence test asserts on.
type civSweepStats struct {
	mediumArchetypeCounts map[string]int
	meanHealth            float64
}

// sweep rolls n ships under the given civ and tallies per-archetype
// frequencies plus the mean of every flight slot's first-unit health.
// Marshals the loadout to JSON to read fields without coupling to
// concrete types.
func sweep(t *testing.T, civ *factory.Civilization, n int) civSweepStats {
	t.Helper()
	stats := civSweepStats{mediumArchetypeCounts: map[string]int{}}
	totalHealth := 0.0
	healthSamples := 0
	for seed := int64(1); seed <= int64(n); seed++ {
		l, err := GenerateRandomShip(seed, civ)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, slot := range []flight.FlightSlot{flight.Short, flight.Medium, flight.Far} {
			sys := l.Flight[slot]
			if sys == nil {
				continue
			}
			b, _ := json.Marshal(sys)
			var info struct {
				ArchetypeName string    `json:"archetype"`
				Health        []float64 `json:"health"`
			}
			if err := json.Unmarshal(b, &info); err != nil {
				continue
			}
			if slot == flight.Medium {
				stats.mediumArchetypeCounts[info.ArchetypeName]++
			}
			for _, h := range info.Health {
				totalHealth += h
				healthSamples++
			}
		}
	}
	if healthSamples > 0 {
		stats.meanHealth = totalHealth / float64(healthSamples)
	}
	return stats
}
