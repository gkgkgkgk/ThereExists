package factory

import "fmt"

type Mixture struct {
	ID              string
	Config          PropellantConfig
	IspMultiplier   float64
	DensityKgM3     float64
	StorabilityDays int  // -1 = indefinite
	Hypergolic      bool // ignites on contact → forces IgnitionMethod = Hypergolic
	Cryogenic       bool // requires active cooling; typically caps restarts

	// IgnitionNeed names the resource required to light this mixture.
	// Nullable iff Hypergolic == true — the dual-direction invariant is
	// documented in Phase 4 Plan §2 but NOT enforced yet (existing
	// mixtures ship with nil and will be filled in a post-infra content
	// pass).
	IgnitionNeed *ResourceID

	// Synthetic flags propellants without a refinery path — antimatter,
	// exotic metastables. Synthetic mixtures are produced by civ-level
	// infrastructure out of scope for Phase 4; no refinery is expected
	// to list them in its Productions.
	Synthetic bool
}

// LookupMixture returns the mixture for the given ID. Safe on an empty
// registry.
func LookupMixture(id string) (*Mixture, bool) {
	m, ok := Mixtures[id]
	return m, ok
}

// Mixtures is the hand-authored propellant registry. Kept small and
// hand-authored because mixtures propagate cross-category: an engine's
// MixtureID must match a tank the ship will later carry (Plan §2).
var Mixtures = map[string]*Mixture{}

func init() {
	reg := func(m *Mixture) { Mixtures[m.ID] = m }

	reg(&Mixture{
		ID:              "LOX_LH2",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     360,
		StorabilityDays: -1,
		Hypergolic:      false,
		Cryogenic:       true,
	})

	reg(&Mixture{
		ID:              "LOX_RP1",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1030,
		StorabilityDays: -1,
		Hypergolic:      false,
		Cryogenic:       true,
	})

	reg(&Mixture{
		ID:              "MMH_NTO",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1190,
		StorabilityDays: 3650, // ~10 years
		Hypergolic:      true,
		Cryogenic:       false,
	})

	reg(&Mixture{
		ID:              "Hydrazine",
		Config:          Monopropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1021,
		StorabilityDays: 3650,
		Hypergolic:      false,
		Cryogenic:       false,
	})

	// Validate every registered mixture. Permissive in Phase 4: an unset
	// IgnitionNeed is legal regardless of Hypergolic, because no content
	// has been authored yet. Once the user fills in ignition requirements
	// the dual-direction invariant (IgnitionNeed==nil iff Hypergolic) will
	// be tightened — see Plan §2 Open Questions.
	for _, m := range Mixtures {
		if m.IgnitionNeed != nil {
			r, ok := LookupResource(*m.IgnitionNeed)
			if !ok {
				panic(fmt.Sprintf("factory: mixture %q references unknown IgnitionNeed %q", m.ID, *m.IgnitionNeed))
			}
			if r.Category != IgnitionComponent && r.Category != Catalyst {
				panic(fmt.Sprintf("factory: mixture %q IgnitionNeed %q has category %s (must be IgnitionComponent or Catalyst)", m.ID, r.ID, r.Category))
			}
		}
	}
}
