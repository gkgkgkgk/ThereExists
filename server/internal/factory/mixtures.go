package factory

import (
	"fmt"
	"log"
	"time"
)

// IgnitionConfig defines what it takes to "spark" a mixture. Bridges
// hardware parts (e.g. a Silver catalyst bed) and consumables (chemical
// starters) under one shape: both are a resource whose Category is
// IgnitionComponent or Catalyst, and both have a per-start cost —
// physical consumption for a chemical starter, wear for a catalyst bed.
type IgnitionConfig struct {
	ID               string
	Resource         *Resource
	QuantityPerStart float64 // consumed (starter) or wear (catalyst) per ignition
	Description      string
}

// IgnitionConfigs is the central registry of ignition configurations.
// Populated from factory/content via RegisterIgnitionConfig at init().
var IgnitionConfigs = map[string]*IgnitionConfig{}

// LookupIgnitionConfig returns the config for the given ID.
func LookupIgnitionConfig(id string) (*IgnitionConfig, bool) {
	ic, ok := IgnitionConfigs[id]
	return ic, ok
}

// RegisterIgnitionConfig adds an ignition config to the registry.
// Validates structure and panics on misauth so authoring mistakes
// surface at startup. Called from content/ files at init().
func RegisterIgnitionConfig(ic *IgnitionConfig) {
	if err := ic.validate(); err != nil {
		panic(fmt.Sprintf("factory: ignition config %q failed validation: %v", ic.ID, err))
	}
	IgnitionConfigs[ic.ID] = ic
}

// Mixture is a refined propellant and its synthesis constraints. One
// chemistry = one canonical recipe; refineries modulate (efficiency,
// throughput) rather than own alternate recipes.
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

	// RefiningTimePerKg is processed against PROPER TIME (τ, ship clock).
	// Duration type is load-bearing — the future runtime loop must
	// explicitly pick τ vs t at each call site rather than drift against
	// an ambient float clock.
	RefiningTimePerKg time.Duration

	// Ignition describes how the mixture is lit, referencing an
	// IgnitionConfig. Must be nil iff Hypergolic. Non-hypergolic
	// mixtures with Ignition == nil log a warning.
	Ignition *IgnitionConfig

	// Synthetic flags propellants without a refinery path — antimatter,
	// exotic metastables. Produced by civ-level infrastructure out of
	// scope here; no refinery should list them in SupportedMixtureIDs.
	// All synthesis fields (Precursors, PowerCostPerKg,
	// RefiningTimePerKg) must be zero when true.
	Synthetic bool
}

// Mixtures is the propellant registry. Populated from factory/content
// via RegisterMixture at init(). Kept on the factory package so flight
// archetypes (and future tank/refinery code) can resolve mixture IDs
// without taking a content/ dependency.
var Mixtures = map[string]*Mixture{}

// LookupMixture returns the mixture for the given ID. Safe on an empty
// registry.
func LookupMixture(id string) (*Mixture, bool) {
	m, ok := Mixtures[id]
	return m, ok
}

// RegisterMixture validates and adds a mixture to the registry.
// Panics on structural inconsistency (Phase 4.1 §4 invariants) so
// authoring mistakes surface at startup. Called from content/ files
// at init().
func RegisterMixture(m *Mixture) {
	if err := m.validate(); err != nil {
		panic(fmt.Sprintf("factory: mixture %q failed validation: %v", m.ID, err))
	}
	Mixtures[m.ID] = m
}

// validate enforces invariants: Synthetic short-circuit, precursor
// category, ignition dual-invariant, power/time monotonicity. Strict
// on internal inconsistencies; logs (not errors) on missing-content
// situations the content pass will fill in later.
func (m *Mixture) validate() error {
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
		return m.validateIgnition()
	}

	for i, ri := range m.Precursors {
		if ri.Resource == nil {
			return fmt.Errorf("Precursors[%d] has nil Resource", i)
		}
		if ri.Resource.Category != WildPrecursor {
			return fmt.Errorf("Precursors[%d] resource %q has category %s (must be WildPrecursor)", i, ri.Resource.DisplayName, ri.Resource.Category)
		}
		if ri.QuantityPerUnitFuel <= 0 {
			return fmt.Errorf("Precursors[%d] QuantityPerUnitFuel %v must be > 0", i, ri.QuantityPerUnitFuel)
		}
	}

	if err := m.validateIgnition(); err != nil {
		return err
	}
	if !m.Hypergolic && m.Ignition == nil {
		log.Printf("factory: mixture %q is non-hypergolic but has no Ignition config — content pass must fill this in", m.ID)
	}

	hasPower := m.PowerCostPerKg > 0
	hasTime := m.RefiningTimePerKg > 0
	if hasPower != hasTime {
		return fmt.Errorf("PowerCostPerKg (%v) and RefiningTimePerKg (%v) must be both zero or both non-zero",
			m.PowerCostPerKg, m.RefiningTimePerKg)
	}
	return nil
}

// validateIgnition: hypergolic mixtures may not declare an Ignition
// config — they light on contact; any external igniter is a
// declaration error.
func (m *Mixture) validateIgnition() error {
	if m.Hypergolic && m.Ignition != nil {
		return fmt.Errorf("hypergolic mixture must not declare an Ignition config (got %q)", m.Ignition.ID)
	}
	return nil
}

// validate ensures the IgnitionConfig itself is structurally sound.
func (ic *IgnitionConfig) validate() error {
	if ic.ID == "" {
		return fmt.Errorf("empty ID")
	}
	if ic.Resource == nil {
		return fmt.Errorf("Resource is nil")
	}
	if ic.QuantityPerStart <= 0 {
		return fmt.Errorf("QuantityPerStart %v must be > 0", ic.QuantityPerStart)
	}
	if ic.Resource.Category != IgnitionComponent && ic.Resource.Category != Catalyst {
		return fmt.Errorf("Resource %q has category %s (must be IgnitionComponent or Catalyst)", ic.Resource.DisplayName, ic.Resource.Category)
	}
	return nil
}
