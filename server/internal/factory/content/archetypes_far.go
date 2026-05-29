package content

import (
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// Relativistic-drive archetype values. Add a new archetype: declare a
// var below and register it in init(). Type definition + generator
// live in factory/flight/far.go.

var RBCABeamCore = flight.RelativisticDriveArchetype{
	Name:        "Relativistic Beam-Core Assembly (RBCA)",
	Description: "Matter/antimatter annihilation in a magnetic nozzle. Pushes the ship to a significant fraction of c at the cost of a gamma signature detectable across light-years. Spawns damaged — the drive is the adventure.",
	FlightSlot:  flight.Far,
	TechTier:    5,
	Rarity:      1.0,

	ThrustIspBias: 1.0,

	HealthInitRange:   [2]float64{0.40, 0.60},
	TopSpeedFractionC: 0.85,

	IspVacuumRange:       [2]float64{1_000_000, 5_000_000},
	ThrustNRange:         [2]float64{500, 2_000},
	OperatingPowerWRange: [2]float64{10_000, 50_000},

	AllowedMixtures: []*factory.Mixture{&Matter_Antimatter_Pair},

	SignatureProfile: "gamma burst, omnidirectional — detectable across light-years",
}

func init() {
	flight.RegisterRelativisticArchetype(RBCABeamCore)
}
