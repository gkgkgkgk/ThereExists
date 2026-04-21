package factory

import (
	"fmt"
	"log"
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

	for _, m := range Mixtures {
		if err := m.validate(); err != nil {
			panic(fmt.Sprintf("factory: mixture %q failed validation: %v", m.ID, err))
		}
	}
}

// validate enforces Phase 4.1 §4 rules. Empty-registry-safe and
// permissive on unset synthesis fields (content is authored post-infra),
// but strict on internal inconsistencies — a half-populated synthesis
// declaration or a hypergolic mixture that also lists an ignition
// resource is a contradiction, not missing content.
func (m *Mixture) validate() error {
	// Rule 1: Synthetic short-circuit. Every synthesis field must be
	// zero — Synthetic means the refinery path is bypassed entirely.
	if m.Synthetic {
		if len(m.Precursors) != 0 {
			return fmt.Errorf("Synthetic mixture must have empty Precursors (got %d)", len(m.Precursors))
		}
		if m.PowerCostPerKg != 0 {
			return fmt.Errorf("Synthetic mixture must have PowerCostPerKg == 0 (got %v)", m.PowerCostPerKg)
		}
		if m.RefiningTimePerKg != 0 {
			return fmt.Errorf("Synthetic mixture must have RefiningTimePerKg == 0 (got %v)", m.RefiningTimePerKg)
		}
		if m.RequiredCatalyst != "" {
			return fmt.Errorf("Synthetic mixture must have empty RequiredCatalyst (got %q)", m.RequiredCatalyst)
		}
		return m.validateIgnition()
	}

	// Rule 2: Precursor category. Every entry must resolve to a
	// WildPrecursor. Partial authoring is not legal — either fully
	// populated or fully empty (empty is the post-Phase-4 default;
	// content pass fills it).
	for i, ri := range m.Precursors {
		if ri.ResourceID == "" {
			return fmt.Errorf("Precursors[%d] has empty ResourceID", i)
		}
		r, ok := LookupResource(ri.ResourceID)
		if !ok {
			return fmt.Errorf("Precursors[%d] unknown resource %q", i, ri.ResourceID)
		}
		if r.Category != WildPrecursor {
			return fmt.Errorf("Precursors[%d] resource %q has category %s (must be WildPrecursor)", i, ri.ResourceID, r.Category)
		}
		if ri.QuantityPerUnitFuel <= 0 {
			return fmt.Errorf("Precursors[%d] QuantityPerUnitFuel %v must be > 0", i, ri.QuantityPerUnitFuel)
		}
	}

	// Rule 3: Catalyst category.
	if m.RequiredCatalyst != "" {
		r, ok := LookupResource(m.RequiredCatalyst)
		if !ok {
			return fmt.Errorf("RequiredCatalyst %q not in resource registry", m.RequiredCatalyst)
		}
		if r.Category != Catalyst {
			return fmt.Errorf("RequiredCatalyst %q has category %s (must be Catalyst)", m.RequiredCatalyst, r.Category)
		}
	}

	// Rule 4: Ignition dual-invariant. Strict on the internal
	// contradiction (hypergolic + ignition resource is nonsense); soft
	// on the missing-content direction (non-hypergolic with no ignition
	// resource is legal until content lands). The soft side logs so the
	// content pass has a concrete to-do list.
	if err := m.validateIgnition(); err != nil {
		return err
	}
	if !m.Hypergolic && m.IgnitionNeed == nil {
		log.Printf("factory: mixture %q is non-hypergolic but has no IgnitionNeed — content pass must fill this in", m.ID)
	}

	// Rule 5: Power/time monotonicity. Claiming power without time, or
	// time without power, is a malformed recipe.
	hasPower := m.PowerCostPerKg > 0
	hasTime := m.RefiningTimePerKg > 0
	if hasPower != hasTime {
		return fmt.Errorf("PowerCostPerKg (%v) and RefiningTimePerKg (%v) must be both zero or both non-zero",
			m.PowerCostPerKg, m.RefiningTimePerKg)
	}
	return nil
}

// validateIgnition handles the strict half of the dual-invariant plus
// category/registry resolution for a declared ignition resource.
// Hypergolic mixtures may not declare an IgnitionNeed — they light on
// contact; any external igniter is a declaration error.
func (m *Mixture) validateIgnition() error {
	if m.Hypergolic && m.IgnitionNeed != nil {
		return fmt.Errorf("hypergolic mixture must not declare IgnitionNeed (got %q)", *m.IgnitionNeed)
	}
	if m.IgnitionNeed == nil {
		return nil
	}
	r, ok := LookupResource(*m.IgnitionNeed)
	if !ok {
		return fmt.Errorf("IgnitionNeed %q not in resource registry", *m.IgnitionNeed)
	}
	if r.Category != IgnitionComponent && r.Category != Catalyst {
		return fmt.Errorf("IgnitionNeed %q has category %s (must be IgnitionComponent or Catalyst)", r.ID, r.Category)
	}
	return nil
}
