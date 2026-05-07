package content

import (
	"time"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// Ignition configurations.

var SparkIgnition = factory.IgnitionConfig{
	ID:               "Spark",
	Resource:         SPARK_RESOURCE,
	QuantityPerStart: 1,
	Description:      "Electric torch igniter; one chemical starter consumed per ignition.",
}

var SilverCatalystIgnition = factory.IgnitionConfig{
	ID:               "SilverCatalyst",
	Resource:         SILVER,
	QuantityPerStart: 0.0005,
	Description:      "Silver catalyst bed; worn slightly on every decomposition start.",
}

var BioScaffoldIgnition = factory.IgnitionConfig{
	ID:               "BioScaffold",
	Resource:         CHON_ICE,
	QuantityPerStart: 0.001,
	Description:      "Enzymatic protein scaffold refined from raw organic CHON precursors.",
}

// Refined propellants. New mixtures: declare a var below, add a
// RegisterMixture call in init(), done.

var Methalox = factory.Mixture{
	ID:              "Methalox",
	Description:     "High-performance cryogenic bipropellant. Offers superior specific impulse for mainline transit but requires active thermal management to prevent boil-off.",
	Config:          factory.Bipropellant,
	IspMultiplier:   1.25,
	DensityKgM3:     830,
	StorabilityDays: 30,
	Hypergolic:      false,
	Cryogenic:       true,
	Precursors: []factory.ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.8},
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.5},
	},
	PowerCostPerKg:    2500.0,
	RefiningTimePerKg: time.Minute * 20,
	Ignition:          &SparkIgnition,
}

var HTP_90 = factory.Mixture{
	ID:              "HTP_90",
	Description:     "90% High-Test Peroxide. A reliable, low-toxicity monopropellant that decomposes into steam and oxygen. Preferred for simplicity and ease of synthesis in the field.",
	Config:          factory.Monopropellant,
	IspMultiplier:   0.75,
	DensityKgM3:     1390,
	StorabilityDays: 180,
	Hypergolic:      false,
	Cryogenic:       false,
	Precursors: []factory.ResourceInput{
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.8},
	},
	PowerCostPerKg:    3000.0,
	RefiningTimePerKg: time.Minute * 10,
	Ignition:          &SilverCatalystIgnition,
}

var MMH_NTO = factory.Mixture{
	ID:              "MMH_NTO",
	Description:     "Standard storable bipropellant. Hypergolic ignition ensures near-instant response for high-frequency RCS pulsing.",
	Config:          factory.Bipropellant,
	IspMultiplier:   1.0,
	DensityKgM3:     1190,
	StorabilityDays: 3650,
	Hypergolic:      true,
	Cryogenic:       false,
	Precursors: []factory.ResourceInput{
		{Resource: NH3_ICE, QuantityPerUnitFuel: 0.8},
		{Resource: N2_ICE, QuantityPerUnitFuel: 0.5},
	},
	PowerCostPerKg:    1800.0,
	RefiningTimePerKg: time.Minute * 12,
	Ignition:          nil,
}

var Aerozine50_NTO = factory.Mixture{
	ID:              "Aerozine50_NTO",
	Description:     "A high-stability hypergolic bipropellant. The workhorse of long-term service modules due to its predictable ignition and indefinite storability.",
	Config:          factory.Bipropellant,
	IspMultiplier:   1.05,
	DensityKgM3:     1120,
	StorabilityDays: 5000,
	Hypergolic:      true,
	Cryogenic:       false,
	Precursors: []factory.ResourceInput{
		{Resource: NH3_ICE, QuantityPerUnitFuel: 1.1},
		{Resource: N2_ICE, QuantityPerUnitFuel: 0.6},
	},
	PowerCostPerKg:    2000.0,
	RefiningTimePerKg: time.Minute * 15,
	Ignition:          nil,
}

var Methane_Fluorine = factory.Mixture{
	ID:              "Methane_Fluorine",
	Description:     "An ultra-energetic detonation mixture. Fluorine's extreme reactivity provides massive thrust-to-weight for RDE manifolds, but produces corrosive hydrogen-fluoride exhaust.",
	Config:          factory.Bipropellant,
	IspMultiplier:   1.6,
	DensityKgM3:     950,
	StorabilityDays: 45,
	Hypergolic:      true,
	Cryogenic:       true,
	Precursors: []factory.ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.7},
		{Resource: FLUORITE_ORE, QuantityPerUnitFuel: 2.2},
	},
	PowerCostPerKg:    5500.0,
	RefiningTimePerKg: time.Minute * 45,
	Ignition:          nil,
}

var CH3OH_Saline_Substrate = factory.Mixture{
	ID:              "CH3OH_Saline_Substrate",
	Description:     "A methanol-perchlorate biogenic substrate. Utilizes Magnesium Perchlorate as a cryoprotectant and electrolyte to maintain the SABRE colony's metabolic potential in deep-space conditions.",
	Config:          factory.Monopropellant,
	IspMultiplier:   1.15,
	DensityKgM3:     1280,
	StorabilityDays: -1,
	Hypergolic:      false,
	Cryogenic:       false,
	Precursors: []factory.ResourceInput{
		{Resource: CH4_ICE, QuantityPerUnitFuel: 0.9},
		{Resource: MG_PERCHLORATE, QuantityPerUnitFuel: 0.3},
	},
	PowerCostPerKg:    1400.0,
	RefiningTimePerKg: time.Hour * 2,
	Ignition:          &BioScaffoldIgnition,
}

var Silane_Ox = factory.Mixture{
	ID:              "Silane_Ox",
	Description:     "Silicon-hydride bipropellant. Highly pyrophoric and easy to refine from common regolith, but the SiO2 exhaust (sand) is abrasive to nozzle geometry.",
	Config:          factory.Bipropellant,
	IspMultiplier:   0.9,
	DensityKgM3:     1050,
	StorabilityDays: 365,
	Hypergolic:      true,
	Cryogenic:       true,
	Precursors: []factory.ResourceInput{
		{Resource: HIGH_Q_SILICATES, QuantityPerUnitFuel: 1.5},
		{Resource: H2O_ICE, QuantityPerUnitFuel: 1.0},
	},
	PowerCostPerKg:    4000.0,
	RefiningTimePerKg: time.Minute * 20,
	Ignition:          nil,
}

// Unauthored placeholders to allow direct object references before
// content pass. They register but archetype validation will skip them.
var Hydrazine = factory.Mixture{ID: "Hydrazine"}
var GlassHydrazine = factory.Mixture{ID: "Glass-Hydrazine"}
var LOX_LH2 = factory.Mixture{ID: "LOX_LH2"}
var Hydrogen_Fluorine = factory.Mixture{ID: "Hydrogen_Fluorine"}
var Polyphosphate_Concentrate = factory.Mixture{ID: "Polyphosphate_Concentrate"}
var Matter_Antimatter_Pair = factory.Mixture{ID: "Matter_Antimatter_Pair", Synthetic: true}

func init() {
	factory.RegisterIgnitionConfig(&SparkIgnition)
	factory.RegisterIgnitionConfig(&SilverCatalystIgnition)
	// BioScaffoldIgnition references CHON_ICE which is a WildPrecursor —
	// IgnitionConfig.validate() rejects WildPrecursors, so it stays
	// referenced from Mixtures but isn't put through RegisterIgnitionConfig.

	factory.RegisterMixture(&Methalox)
	factory.RegisterMixture(&HTP_90)
	factory.RegisterMixture(&MMH_NTO)
	factory.RegisterMixture(&Aerozine50_NTO)
	factory.RegisterMixture(&Methane_Fluorine)
	factory.RegisterMixture(&CH3OH_Saline_Substrate)
	factory.RegisterMixture(&Silane_Ox)
	factory.RegisterMixture(&Hydrazine)
	factory.RegisterMixture(&GlassHydrazine)
	factory.RegisterMixture(&LOX_LH2)
	factory.RegisterMixture(&Hydrogen_Fluorine)
	factory.RegisterMixture(&Polyphosphate_Concentrate)
	factory.RegisterMixture(&Matter_Antimatter_Pair)
}
