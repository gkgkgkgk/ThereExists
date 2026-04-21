package flight

// Relativistic-drive archetype values live here. Add new entries by
// declaring another var below and registering it in init(). Phase 4
// ships one archetype: RBCA, gated to TechTier 5. See Phase 4 Plan §1.

var RBCABeamCore = RelativisticDriveArchetype{
	Name:        "Relativistic Beam-Core Assembly (RBCA)",
	Description: "Matter/antimatter annihilation in a magnetic nozzle. Pushes the ship to a significant fraction of c at the cost of a gamma signature detectable across light-years. Spawns damaged — the drive is the adventure.",
	FlightSlot:  Far,
	TechTier:    5,

	HealthInitRange:   [2]float64{0.40, 0.60},
	TopSpeedFractionC: 0.85,

	IspVacuumRange:       [2]float64{1_000_000, 5_000_000},
	ThrustNRange:         [2]float64{500, 2_000},
	OperatingPowerWRange: [2]float64{10_000, 50_000},

	AllowedMixtureIDs: []string{"Matter_Antimatter_Pair"},

	SignatureProfile: "gamma burst, omnidirectional — detectable across light-years",
}

func init() {
	registerRelativisticArchetype(RBCABeamCore)
}
