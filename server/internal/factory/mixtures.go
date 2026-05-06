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
var IgnitionConfigs = map[string]*IgnitionConfig{}

// LookupIgnitionConfig returns the config for the given ID.
func LookupIgnitionConfig(id string) (*IgnitionConfig, bool) {
	ic, ok := IgnitionConfigs[id]
	return ic, ok
}

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

	// Ignition describes how the mixture is lit, referencing an IgnitionConfig.
	// Must be nil iff Hypergolic (dual-invariant from Phase 4.1 §4).
	// Non-hypergolic mixtures with Ignition == nil log a warning.
	Ignition *IgnitionConfig

	// Synthetic flags propellants without a refinery path — antimatter,
	// exotic metastables. Produced by civ-level infrastructure out of
	// scope here; no refinery should list them in SupportedMixtureIDs.
	// All synthesis fields (Precursors, PowerCostPerKg,
	// RefiningTimePerKg) must be zero when true.
	Synthetic bool
}

// LookupMixture returns the mixture for the given ID. Safe on an empty
// registry.
func LookupMixture(id string) (*Mixture, bool) {
	m, ok := Mixtures[id]
	return m, ok
}

var SparkIgnition = IgnitionConfig{
	ID:               "Spark",
	Resource:         SPARK_RESOURCE,
	QuantityPerStart: 1,
	Description:      "Electric torch igniter; one chemical starter consumed per ignition.",
}

var SilverCatalystIgnition = IgnitionConfig{
	ID:               "SilverCatalyst",
	Resource:         SILVER,
	QuantityPerStart: 0.0005,
	Description:      "Silver catalyst bed; worn slightly on every decomposition start.",
}

var Bio_Scaffold_Ignition = IgnitionConfig{
	ID:               "BioScaffold",
	Resource:         CHON_ICE, // Find bio-gunk, grow a spark
	QuantityPerStart: 0.001,    // "Denaturation" or biological atrophy per start
	Description:      "Enzymatic protein scaffold refined from raw organic CHON precursors.",
}

var Methalox = Mixture{
	ID:              "Methalox",
	Description:     "High-performance cryogenic bipropellant. Offers superior specific impulse for mainline transit but requires active thermal management to prevent boil-off.",
	Config:          Bipropellant,
	IspMultiplier:   1.25,
	DensityKgM3:     830,
	StorabilityDays: 30,
	Hypergolic:      false,
	Cryogenic:       true,

	Precursors: []ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.8},
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.5},
	},

	PowerCostPerKg:    2500.0,
	RefiningTimePerKg: time.Minute * 20,

	Ignition: &SparkIgnition,
}

var HTP_90 = Mixture{
	ID:              "HTP_90",
	Description:     "90% High-Test Peroxide. A reliable, low-toxicity monopropellant that decomposes into steam and oxygen. Preferred for simplicity and ease of synthesis in the field.",
	Config:          Monopropellant,
	IspMultiplier:   0.75,
	DensityKgM3:     1390,
	StorabilityDays: 180,
	Hypergolic:      false,
	Cryogenic:       false,

	Precursors: []ResourceInput{
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.8},
	},

	PowerCostPerKg:    3000.0,
	RefiningTimePerKg: time.Minute * 10,

	// Catalytic engines don't need a spark — they need the Silver
	// catalyst bed to be intact. QuantityPerStart is wear, not
	// consumption.
	Ignition: &SilverCatalystIgnition,
}

var MMH_NTO = Mixture{
	ID:              "MMH_NTO",
	Description:     "Standard storable bipropellant. Hypergolic ignition ensures near-instant response for high-frequency RCS pulsing.",
	Config:          Bipropellant,
	IspMultiplier:   1.0,
	DensityKgM3:     1190,
	StorabilityDays: 3650,
	Hypergolic:      true,
	Cryogenic:       false,

	Precursors: []ResourceInput{
		{Resource: NH3_ICE, QuantityPerUnitFuel: 0.8},
		{Resource: N2_ICE, QuantityPerUnitFuel: 0.5},
	},

	PowerCostPerKg:    1800.0,
	RefiningTimePerKg: time.Minute * 12,

	// Hypergolic: lights on contact; no external ignition.
	Ignition: nil,
}

var Aerozine50_NTO = Mixture{
	ID:              "Aerozine50_NTO",
	Description:     "A high-stability hypergolic bipropellant. The workhorse of long-term service modules due to its predictable ignition and indefinite storability.",
	Config:          Bipropellant,
	IspMultiplier:   1.05,
	DensityKgM3:     1120,
	StorabilityDays: 5000, // Almost forever
	Hypergolic:      true,
	Cryogenic:       false,

	Precursors: []ResourceInput{
		{Resource: NH3_ICE, QuantityPerUnitFuel: 1.1}, // For the hydrazine component
		{Resource: N2_ICE, QuantityPerUnitFuel: 0.6},  // For the nitrogen tetroxide
	},

	PowerCostPerKg:    2000.0,
	RefiningTimePerKg: time.Minute * 15,

	Ignition: nil,
}

var Methane_Fluorine = Mixture{
	ID:              "Methane_Fluorine",
	Description:     "An ultra-energetic detonation mixture. Fluorine's extreme reactivity provides massive thrust-to-weight for RDE manifolds, but produces corrosive hydrogen-fluoride exhaust.",
	Config:          Bipropellant,
	IspMultiplier:   1.6,
	DensityKgM3:     950,
	StorabilityDays: 45, // Corrodes tank liners over time
	Hypergolic:      true,
	Cryogenic:       true,

	Precursors: []ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.7},
		{Resource: FLUORITE_ORE, QuantityPerUnitFuel: 2.2}, // High mass-loss during extraction
	},

	PowerCostPerKg:    5500.0, // Electrolyzing Fluorine is a power hog
	RefiningTimePerKg: time.Minute * 45,

	// Hypergolic = No ignition needed
	Ignition: nil,
}

var CH3OH_Saline_Substrate = Mixture{
	ID:              "CH3OH_Saline_Substrate",
	Description:     "A methanol-perchlorate biogenic substrate. Utilizes Magnesium Perchlorate as a cryoprotectant and electrolyte to maintain the SABRE colony's metabolic potential in deep-space conditions.",
	Config:          Monopropellant,
	IspMultiplier:   1.15,
	DensityKgM3:     1280, // Perchlorates increase fluid density
	StorabilityDays: -1,
	Hypergolic:      false,
	Cryogenic:       false,

	Precursors: []ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.9},        // For the Methanol
		{Resource: MG_PERCHLORATE, QuantityPerUnitFuel: 0.3}, // For the Saline conductivity
	},

	PowerCostPerKg:    1400.0,
	RefiningTimePerKg: time.Hour * 2, // Balancing growth and chemistry

	Ignition: &Bio_Scaffold_Ignition,
}

var Silane_Ox = Mixture{
	ID:              "Silane_Ox",
	Description:     "Silicon-hydride bipropellant. Highly pyrophoric and easy to refine from common regolith, but the SiO2 exhaust (sand) is abrasive to nozzle geometry.",
	Config:          Bipropellant,
	IspMultiplier:   0.9, // Low performance, but 'infinite' ammo
	DensityKgM3:     1050,
	StorabilityDays: 365,
	Hypergolic:      true, // Ignites on contact with LOX
	Cryogenic:       true, // Oxygen component must be kept cold

	Precursors: []ResourceInput{
		{Resource: HIGH_Q_SILICATES, QuantityPerUnitFuel: 1.5},
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.0}, // For the Oxygen oxidizer
	},

	PowerCostPerKg:    4000.0, // Refining Silicon takes a lot of juice
	RefiningTimePerKg: time.Minute * 20,

	Ignition: nil,
}

// Unauthored placeholders to allow direct object references before content pass.
var Hydrazine = Mixture{ID: "Hydrazine"}
var GlassHydrazine = Mixture{ID: "Glass-Hydrazine"}
var LOX_LH2 = Mixture{ID: "LOX_LH2"}
var Hydrogen_Fluorine = Mixture{ID: "Hydrogen_Fluorine"}
var Polyphosphate_Concentrate = Mixture{ID: "Polyphosphate_Concentrate"}
var Matter_Antimatter_Pair = Mixture{ID: "Matter_Antimatter_Pair", Synthetic: true}

// Mixtures is the hand-authored propellant registry. Kept small and
// hand-authored because mixtures propagate cross-category: an engine's
// MixtureID must match a tank the ship will later carry (Plan §2).
var Mixtures = map[string]*Mixture{}

func init() {
	regIC := func(ic *IgnitionConfig) { IgnitionConfigs[ic.ID] = ic }
	regIC(&SparkIgnition)
	regIC(&SilverCatalystIgnition)

	for _, ic := range IgnitionConfigs {
		if err := ic.validate(); err != nil {
			panic(fmt.Sprintf("factory: ignition config %q failed validation: %v", ic.ID, err))
		}
	}

	regM := func(m *Mixture) { Mixtures[m.ID] = m }

	regM(&Methalox)
	regM(&HTP_90)
	regM(&MMH_NTO)
	regM(&Aerozine50_NTO)
	regM(&Methane_Fluorine)
	regM(&CH3OH_Saline_Substrate)
	regM(&Silane_Ox)
	regM(&Hydrazine)
	regM(&GlassHydrazine)
	regM(&LOX_LH2)
	regM(&Hydrogen_Fluorine)
	regM(&Polyphosphate_Concentrate)
	regM(&Matter_Antimatter_Pair)

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
		return m.validateIgnition()
	}

	// Rule 2: Precursor category. Every entry must resolve to a
	// WildPrecursor. Partial authoring is not legal — either fully
	// populated or fully empty (empty is the post-Phase-4 default;
	// content pass fills it).
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

	// Rule 4: Ignition dual-invariant. Strict on the internal
	// contradiction (hypergolic + ignition config is nonsense); soft on
	// the missing-content direction (non-hypergolic with no IgnitionID is
	// legal until content lands). The soft side logs so the content
	// pass has a concrete to-do list.
	if err := m.validateIgnition(); err != nil {
		return err
	}
	if !m.Hypergolic && m.Ignition == nil {
		log.Printf("factory: mixture %q is non-hypergolic but has no Ignition config — content pass must fill this in", m.ID)
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

// validateIgnition handles the strict half of the dual-invariant.
// Hypergolic mixtures may not declare an Ignition config — they light on
// contact; any external igniter is a declaration error.
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
