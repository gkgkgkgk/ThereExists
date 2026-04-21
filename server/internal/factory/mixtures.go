package factory

import (
	"fmt"
	"time"
)

// Mixture is a refined propellant and its synthesis constraints. Phase
// 4.1 promoted the recipe onto the mixture itself (was on
// refinery.MixtureProduction in Phase 4) so one chemistry = one
// canonical recipe; refineries modulate (efficiency, throughput) rather
// than own alternate recipes. See Phase4_1_Plan.md §1.
type Mixture struct {
	ID              string
	Description     string
	Config          PropellantConfig
	IspMultiplier   float64
	DensityKgM3     float64
	StorabilityDays int  // -1 = indefinite
	Hypergolic      bool // ignites on contact → forces IgnitionMethod = Hypergolic
	Cryogenic       bool // requires active cooling; typically caps restarts

	// Synthesis — the "metabolic" layer. Canonical and single-path.
	// Precursors lists WILD PRECURSOR inputs per kg of finished
	// propellant; refineries apply Efficiency to modulate actual
	// feedstock consumption. Refined chemicals are never resources and
	// cannot appear here. Empty on synthetic mixtures (see Synthetic).
	Precursors []ResourceInput

	// PowerCostPerKg is the continuous watts the chemistry demands
	// during refining. Scalar — the refinery's own power envelope is
	// separate (idle draw, efficiency losses).
	PowerCostPerKg float64

	// RefiningTimePerKg is processed against PROPER TIME (τ, ship
	// clock). See TE_TimeDilation.md. Duration type is load-bearing —
	// the future runtime loop must explicitly pick τ vs t at each call
	// site rather than drift against an ambient float clock.
	RefiningTimePerKg time.Duration

	// RequiredCatalyst is a hardware consumable that wears down as the
	// refinery runs (wear lives on Refinery.CatalystHealth, not here).
	// Empty string means catalyst-free chemistry.
	RequiredCatalyst ResourceID

	// IgnitionNeed names the resource required to light this mixture.
	// Phase 4.1 tightens the invariant: must be nil iff Hypergolic.
	IgnitionNeed *ResourceID

	// Synthetic flags propellants without a refinery path — antimatter,
	// exotic metastables. Produced by civ-level infrastructure out of
	// scope here; no refinery should list them in SupportedMixtureIDs.
	// All synthesis fields (Precursors, PowerCostPerKg,
	// RefiningTimePerKg, RequiredCatalyst) must be zero when true.
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

	// Matter_Antimatter_Pair — synthetic, produced by civ-level infrastructure
	// out of scope for Phase 4. Bypasses the refinery (Synthetic: true).
	// StorabilityDays: 0 signals continuous containment power required.
	// IgnitionNeed stays nil until Magnetic_Trap_Assembly is authored in the
	// content pass; post-infra the pointer will be set explicitly.
	reg(&Mixture{
		ID:              "Matter_Antimatter_Pair",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     100,
		StorabilityDays: 0,
		Hypergolic:      false,
		Cryogenic:       true,
		Synthetic:       true,
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
