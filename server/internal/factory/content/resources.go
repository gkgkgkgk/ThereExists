// Package content owns every authored object the factory ships with.
// All catalog data — resources, mixtures, ignition configs, flight
// archetypes — lives here. Each file in this package is a place a
// content author goes to add new entries. The package's init()
// chain registers everything into the factory and factory/flight
// registries via the Register* functions those packages export.
//
// Wiring: main.go imports this package as a blank import
// (`_ "...factory/content"`) so Go's init order guarantees content
// is registered before the first request runs.
package content

import "github.com/gkgkgkgk/ThereExists/server/internal/factory"

// Wild precursors — volatiles harvested from comets, asteroids, and
// icy moons. QuantityPerUnitFuel in Mixture.Precursors uses the same
// units (kg per kg of finished propellant).

var H2O_ICE = &factory.Resource{
	DisplayName:       "Water Ice",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       1,
	TypicalSourceHint: "Cometary ice, outer-system moons, shadowed craters.",
}

var CH4_ICE = &factory.Resource{
	DisplayName:       "Methane Ice",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       2,
	TypicalSourceHint: "Outer-system bodies; Titan-class moons; cold comet cores.",
}

var MG_PERCHLORATE = &factory.Resource{
	DisplayName:       "Magnesium Perchlorate",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       4,
	TypicalSourceHint: "Hyper-arid planetary regolith; evaporite basins on Jovian moons.",
}

var FLUORITE_ORE = &factory.Resource{
	DisplayName:       "Fluorite Ore",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       5,
	TypicalSourceHint: "Deep-vein mining on tectonically active asteroids; rare volcanic plumes.",
}

var HIGH_Q_SILICATES = &factory.Resource{
	DisplayName:       "High-Q Crystalline Silicates",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       1,
	TypicalSourceHint: "Exposed crystalline outcrops; quartz-rich asteroid veins.",
}

var NH3_ICE = &factory.Resource{
	DisplayName:       "Ammonia Ice",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       3,
	TypicalSourceHint: "Cryovolcanic moons; outer-belt cometary inclusions.",
}

var N2_ICE = &factory.Resource{
	DisplayName:       "Nitrogen Ice",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       3,
	TypicalSourceHint: "Cold outer-system bodies; Pluto-class surfaces.",
}

var CHON_ICE = &factory.Resource{
	DisplayName:       "CHON-Rich Ice",
	Category:          factory.WildPrecursor,
	Phase:             factory.Solid,
	Commonality:       3,
	TypicalSourceHint: "Dark-red 'Tholin' crusts on outer-system comets; organic-rich planetesimals.",
}

// Ignition hardware. SPARK is a consumable chemical starter; SILVER
// is a catalyst bed (wears rather than is consumed, but category-wise
// it's still a Catalyst — IgnitionConfig treats both the same and
// QuantityPerStart carries the wear-vs-consume amount).

var SPARK_RESOURCE = &factory.Resource{
	DisplayName:       "Chemical Starter Cartridge",
	Category:          factory.IgnitionComponent,
	Phase:             factory.Solid,
	Commonality:       2,
	TypicalSourceHint: "Standard-issue igniter stock; manufactured, not harvested.",
}

var SILVER = &factory.Resource{
	DisplayName:       "Silver Catalyst Bed",
	Category:          factory.Catalyst,
	Phase:             factory.Solid,
	Commonality:       3,
	TypicalSourceHint: "Refined silver; salvaged from asteroid-belt processing or pre-authored hardware stock.",
}
