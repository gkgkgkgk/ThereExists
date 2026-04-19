package factory

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// Phase 3 manufacturer roster. Placeholder names — a later HIL pass
// replaces these. All entries belong to GenericCivilization; Phase 4's
// LLM civ generation fans them out across cultures.

func init() {
	reg := func(m *Manufacturer) { Manufacturers[m.ID] = m }

	reg(&Manufacturer{
		ID:             "kirov_rocketworks",
		CivilizationID: GenericCivilizationID,
		DisplayName:    "Kirov Rocketworks",
		NamingConvention: func(rng *rand.Rand, archetype string) string {
			return fmt.Sprintf("KR-%s-%04d", shortCode(archetype), rng.Intn(9000)+1000)
		},
		Flavor: "Heavy-industry propulsion house. Conservative, reliable, unexciting.",
	})

	reg(&Manufacturer{
		ID:             "helios_propulsion",
		CivilizationID: GenericCivilizationID,
		DisplayName:    "Helios Propulsion",
		NamingConvention: func(rng *rand.Rand, archetype string) string {
			return fmt.Sprintf("HP-%s-%04d", shortCode(archetype), rng.Intn(9000)+1000)
		},
		Flavor: "Performance-first bureau. Tight tolerances, unforgiving envelopes.",
	})

	reg(&Manufacturer{
		ID:             "triton_dynamics",
		CivilizationID: GenericCivilizationID,
		DisplayName:    "Triton Dynamics",
		NamingConvention: func(rng *rand.Rand, archetype string) string {
			return fmt.Sprintf("TD-%s-%04d", shortCode(archetype), rng.Intn(9000)+1000)
		},
		Flavor: "Mid-market generalist. Nothing brilliant; nothing broken.",
	})

	// Wire the flight dispatcher to our manufacturer picker. Must happen
	// after Manufacturers is populated — Go init() order within a package
	// follows source-file dependency order, and this file imports
	// flight/, so Civilizations/Manufacturers literals above are already
	// populated when this call runs.
	flight.SetManufacturerPicker(pickManufacturer)
}

// shortCode returns a compact archetype code for serial numbers.
// "RCSLiquidChemical" → "RCS-LC".
func shortCode(archetype string) string {
	switch archetype {
	case "RCSLiquidChemical":
		return "RCS-LC"
	}
	return archetype
}

// pickManufacturer implements the injected picker for flight.GenerateForSlot.
// Filter manufacturers by civilization, weight by archetype, sample.
// A nil or missing archetype weight defaults to 1.0.
func pickManufacturer(civilizationID, archetypeName string, rng *rand.Rand) (string, error) {
	type candidate struct {
		id     string
		weight float64
	}
	var cands []candidate
	total := 0.0

	// Stable iteration order so the weighted sample is deterministic for
	// a given rng seed.
	ids := make([]string, 0, len(Manufacturers))
	for id := range Manufacturers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		m := Manufacturers[id]
		if m.CivilizationID != civilizationID {
			continue
		}
		w := 1.0
		if m.ArchetypeWeights != nil {
			if wt, ok := m.ArchetypeWeights[archetypeName]; ok {
				w = wt
			}
		}
		if w <= 0 {
			continue
		}
		cands = append(cands, candidate{id, w})
		total += w
	}

	if len(cands) == 0 {
		return "", errors.New("no manufacturer available for civilization " + civilizationID)
	}

	r := rng.Float64() * total
	acc := 0.0
	for _, c := range cands {
		acc += c.weight
		if r < acc {
			return c.id, nil
		}
	}
	return cands[len(cands)-1].id, nil
}
