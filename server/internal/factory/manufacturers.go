package factory

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
)

type Manufacturer struct {
	ID               string
	CivilizationID   string
	DisplayName      string
	NamingConvention func(rng *rand.Rand, archetype string) string
	ArchetypeWeights map[string]float64
	Flavor           string
}

// Manufacturers is the hand-authored registry. Phase 3 roster is three
// placeholders all belonging to GenericCivilization; a later HIL pass
// replaces these with authored names/styles.
var Manufacturers = map[string]*Manufacturer{}

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

// SameManufacturerBias multiplies a candidate's weight when it matches
// the manufacturer chosen for a previous slot on the same ship. Soft
// pull, not a hard rule — a manufacturer that doesn't make the current
// archetype (weight 0) is still passed over. Value picked so that on
// a three-manufacturer roster with equal base weights the prior
// manufacturer wins ~60% of the time.
const SameManufacturerBias = 3.0

// PickManufacturer implements the picker contract consumed by
// flight.SetManufacturerPicker. Filter by civilization, weight by
// archetype, sample. A nil or missing archetype weight defaults to 1.0.
// If previousManufacturerID is non-empty and appears in the candidate
// list with a positive weight, its weight is multiplied by
// SameManufacturerBias so subsequent slots tend to come from the same
// manufacturer (soft pull — zero-weight candidates stay excluded).
// Exported so main.go can wire it into flight/ at startup without
// creating a factory → flight → factory import cycle.
func PickManufacturer(civilizationID, archetypeName, previousManufacturerID string, rng *rand.Rand) (string, error) {
	type candidate struct {
		id     string
		weight float64
	}
	var cands []candidate
	total := 0.0

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
		if id == previousManufacturerID {
			w *= SameManufacturerBias
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
