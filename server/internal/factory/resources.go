package factory

import "fmt"

// ResourceCategory classifies what role a resource plays in the metabolic
// loop. Wild precursors are raw mass found in the void; catalysts wear
// with use (either in a refinery or an engine); ignition components are
// one-shot-ish hardware needed to light a burn. Refined chemicals
// (LOX, LH2, MMH, etc.) are NOT resources — they are mixtures produced
// by refineries from wild precursors.
type ResourceCategory int

const (
	WildPrecursor ResourceCategory = iota
	Catalyst
	IgnitionComponent
)

func (c ResourceCategory) String() string {
	switch c {
	case WildPrecursor:
		return "WildPrecursor"
	case Catalyst:
		return "Catalyst"
	case IgnitionComponent:
		return "IgnitionComponent"
	default:
		return fmt.Sprintf("ResourceCategory(%d)", int(c))
	}
}

type PhaseOfMatter int

const (
	Solid PhaseOfMatter = iota
	Liquid
	Gas
	Plasma
	Exotic
)

func (p PhaseOfMatter) String() string {
	switch p {
	case Solid:
		return "Solid"
	case Liquid:
		return "Liquid"
	case Gas:
		return "Gas"
	case Plasma:
		return "Plasma"
	case Exotic:
		return "Exotic"
	default:
		return fmt.Sprintf("PhaseOfMatter(%d)", int(p))
	}
}

type Resource struct {
	DisplayName       string
	Category          ResourceCategory
	Phase             PhaseOfMatter
	Commonality       int // 1..5; 1 = ubiquitous, 5 = effectively unobtainable
	TypicalSourceHint string
}

// ResourceInput is a (resource, per-kg quantity) pair. Used by refinery
// productions to declare wild-precursor inputs per kg of finished
// propellant. Lives in the factory package (rather than refinery) so
// non-refinery consumers can reference it without importing refinery.
type ResourceInput struct {
	Resource            *Resource
	QuantityPerUnitFuel float64
}

// Wild precursors — volatiles harvested from comets, asteroids, and
// icy moons. QuantityPerUnitFuel in Mixture.Precursors is in the
// same units (kg-ish per kg of finished propellant).
var H2O_ICE = &Resource{
	DisplayName:       "Water Ice",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       1,
	TypicalSourceHint: "Cometary ice, outer-system moons, shadowed craters.",
}

var CH4_ICE = &Resource{
	DisplayName:       "Methane Ice",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       2,
	TypicalSourceHint: "Outer-system bodies; Titan-class moons; cold comet cores.",
}

var MG_PERCHLORATE = &Resource{
	DisplayName:       "Magnesium Perchlorate",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       4, // Less common than H2O, but localized
	TypicalSourceHint: "Hyper-arid planetary regolith; evaporite basins on Jovian moons.",
}

var FLUORITE_ORE = &Resource{
	DisplayName:       "Fluorite Ore",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       5, // Rare!
	TypicalSourceHint: "Deep-vein mining on tectonically active asteroids; rare volcanic plumes.",
}

var HIGH_Q_SILICATES = &Resource{
	DisplayName:       "High-Q Crystalline Silicates",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       1, // It's literally everywhere (Quartz/Sand)
	TypicalSourceHint: "Exposed crystalline outcrops; quartz-rich asteroid veins.",
}

var NH3_ICE = &Resource{
	DisplayName:       "Ammonia Ice",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       3,
	TypicalSourceHint: "Cryovolcanic moons; outer-belt cometary inclusions.",
}

var N2_ICE = &Resource{
	DisplayName:       "Nitrogen Ice",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       3,
	TypicalSourceHint: "Cold outer-system bodies; Pluto-class surfaces.",
}

// Ignition hardware. SPARK is a consumable chemical starter;
// SILVER is a catalyst bed (wears rather than is consumed, but
// category-wise it's still a Catalyst — IgnitionConfig treats both
// the same and QuantityPerStart carries the wear-vs-consume amount).
var SPARK_RESOURCE = &Resource{
	DisplayName:       "Chemical Starter Cartridge",
	Category:          IgnitionComponent,
	Phase:             Solid,
	Commonality:       2,
	TypicalSourceHint: "Standard-issue igniter stock; manufactured, not harvested.",
}

var SILVER = &Resource{
	DisplayName:       "Silver Catalyst Bed",
	Category:          Catalyst,
	Phase:             Solid,
	Commonality:       3,
	TypicalSourceHint: "Refined silver; salvaged from asteroid-belt processing or pre-authored hardware stock.",
}

var CHON_ICE = &Resource{
	DisplayName:       "CHON-Rich Ice",
	Category:          WildPrecursor,
	Phase:             Solid,
	Commonality:       3,
	TypicalSourceHint: "Dark-red 'Tholin' crusts on outer-system comets; organic-rich planetesimals.",
}
