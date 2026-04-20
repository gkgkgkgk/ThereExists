package flight

import (
	"fmt"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// Liquid-chemical archetype *values* live here. Add a new archetype by
// declaring another var below and registering it in the init() block.
// Struct definitions, validation, and the generator live in liquid.go.

// RCSLiquidChemical — Phase 3's one end-to-end archetype. Values match
// Plan §2 "Example archetype (v1)". Close-range reaction-control engines:
// on/off throttle, unlimited restarts, low chamber pressure.
var RCSLiquidChemical = LiquidChemicalArchetype{
	Name:                            "RCSLiquidChemical",
	FlightSlot:                      Short,
	HealthInitRange:                 [2]float64{0.85, 1.0},
	CountRange:                      [2]int{4, 16}, // RCS clusters are many small thrusters
	ChamberPressureRange:            [2]float64{5, 25},
	IspVacuumRange:                  [2]float64{220, 290},
	IspAtRefPressureRange:           [2]float64{180, 240},
	ReferencePressurePa:             101325,
	ThrustVacuumRange:               [2]float64{50, 1_000}, // log-uniform
	DryMassRange:                    [2]float64{1, 50},     // log-uniform
	GimbalEligibleMassKg:            9999,                  // RCS never gimbals
	GimbalRangeRange:                [2]float64{0, 0},
	IgnitionPowerWRange:             [2]float64{0, 20},
	OperatingPowerWRange:            [2]float64{5, 50},
	AllowedMixtureIDs:               []string{"MMH_NTO", "Hydrazine"},
	AllowedCoolingMethods:           []factory.CoolingMethod{factory.Ablative, factory.Radiative, factory.Film},
	MaxContinuousBurnRange:          [2]float64{1, 300},
	MaxRestarts:                     -1,
	MinThrottleRange:                [2]float64{1.0, 1.0}, // on/off
	MaxThrottleRange:                [2]float64{1.0, 1.0},
	RestartTemperatureCeilingKRange: [2]float64{400, 600},
	AblatorMassKgRange:              [2]float64{0.1, 2.0},
}

func init() {
	registerLiquidArchetype(RCSLiquidChemical)
	// Validate every registered archetype at package load so a bad
	// archetype trips server startup, not a random request.
	for _, a := range registeredArchetypes {
		if err := a.Validate(); err != nil {
			panic(fmt.Sprintf("flight: archetype %q failed validation: %v", a.Name, err))
		}
	}
}
