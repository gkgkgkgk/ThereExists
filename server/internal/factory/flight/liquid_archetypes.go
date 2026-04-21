package flight

import (
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// Liquid-chemical archetype *values* live here. Add a new archetype by
// declaring another var below and registering it in the init() block.
// Struct definitions, validation, and the generator live in liquid.go.

// RCSLiquidChemical — Phase 3's one end-to-end archetype. Values match
// Plan §2 "Example archetype (v1)". Close-range reaction-control engines:
// on/off throttle, unlimited restarts, low chamber pressure.
// liquid_archetypes.go

var RCAStandard = LiquidChemicalArchetype{
	Name:                            "Reaction Control Assembly (RCA)",
	Description:                     "Standard-issue reaction control thrusters. Optimized for attitude control and precision station-keeping via high-frequency pulsing.",
	FlightSlot:                      Short,
	Rarity:                          1.0,
	HealthInitRange:                 [2]float64{0.85, 1.0},
	CountRange:                      [2]int{4, 16}, // RCS clusters are many small thrusters
	ChamberPressureRange:            [2]float64{5, 25},
	IspVacuumRange:                  [2]float64{220, 290},
	IspAtRefPressureRange:           [2]float64{180, 240},
	ReferencePressurePa:             101325,
	ThrustVacuumRange:               [2]float64{50, 1_000},
	DryMassRange:                    [2]float64{1, 50},
	GimbalEligibleMassKg:            9999, // Never gimbals
	GimbalRangeRange:                [2]float64{0, 0},
	IgnitionPowerWRange:             [2]float64{0, 20},
	OperatingPowerWRange:            [2]float64{5, 50},
	AllowedMixtureIDs:               []string{"MMH_NTO", "Hydrazine"},
	AllowedCoolingMethods:           []factory.CoolingMethod{factory.Ablative, factory.Radiative, factory.Film},
	MaxContinuousBurnRange:          [2]float64{1, 300},
	MaxRestarts:                     -1, // Unlimited pulses
	MinThrottleRange:                [2]float64{1.0, 1.0},
	MaxThrottleRange:                [2]float64{1.0, 1.0},
	RestartTemperatureCeilingKRange: [2]float64{400, 600},
	AblatorMassKgRange:              [2]float64{0.1, 2.0},
}

var TCAShort = LiquidChemicalArchetype{
	Name:                   "Thermal Catalytic Assembly (TCA)",
	Description:            "A low-complexity monopropellant thruster. Uses a catalyst bed to decompose propellant into high-temperature steam. Extremely reliable for attitude control and translation, though highly inefficient compared to bipropellant systems.",
	FlightSlot:             Short,
	Rarity:                 0.8,
	HealthInitRange:        [2]float64{0.95, 1.0},
	CountRange:             [2]int{8, 20}, // reliable monoprop cluster
	ChamberPressureRange:   [2]float64{5, 15},
	IspVacuumRange:         [2]float64{140, 190},
	IspAtRefPressureRange:  [2]float64{100, 150},
	ReferencePressurePa:    101325,
	ThrustVacuumRange:      [2]float64{20, 800},
	DryMassRange:           [2]float64{5, 30},
	GimbalEligibleMassKg:   9999, // monoprop cluster never gimbals
	GimbalRangeRange:       [2]float64{0, 0},
	IgnitionPowerWRange:    [2]float64{0, 10},
	OperatingPowerWRange:   [2]float64{2, 15},
	AllowedMixtureIDs:      []string{"Hydrazine"}, // "Hydrazine_Mono" renamed
	AllowedCoolingMethods:  []factory.CoolingMethod{factory.Radiative},
	MaxRestarts:            -1,
	MaxContinuousBurnRange: [2]float64{500, 2000},
	MinThrottleRange:       [2]float64{1.0, 1.0},
	MaxThrottleRange:       [2]float64{1.0, 1.0},
}

var PDDPhotolytic = LiquidChemicalArchetype{
	Name:                   "Photolytic Decomposition Drive (PDD)",
	Description:            "Solid-state manifold using UV-laser dissociation. Eliminates combustion instability but introduces significant electrical overhead during operation.",
	FlightSlot:             Short,
	Rarity:                 0.25,
	HealthInitRange:        [2]float64{0.75, 0.90},
	CountRange:             [2]int{2, 6},
	ChamberPressureRange:   [2]float64{15, 60},
	IspVacuumRange:         [2]float64{380, 410},
	IspAtRefPressureRange:  [2]float64{320, 360},
	ReferencePressurePa:    101325,
	ThrustVacuumRange:      [2]float64{30, 600},
	DryMassRange:           [2]float64{8, 40},
	GimbalEligibleMassKg:   9999,
	GimbalRangeRange:       [2]float64{0, 0},
	IgnitionPowerWRange:    [2]float64{500, 1500},
	OperatingPowerWRange:   [2]float64{400, 1200},
	AllowedMixtureIDs:      []string{"Glass-Hydrazine"}, // unauthored — warn-and-skip at registration
	AllowedCoolingMethods:  []factory.CoolingMethod{factory.Radiative},
	MaxRestarts:            -1,
	MaxContinuousBurnRange: [2]float64{100, 800},
	MinThrottleRange:       [2]float64{1.0, 1.0},
	MaxThrottleRange:       [2]float64{1.0, 1.0},
}

var HPFAService = LiquidChemicalArchetype{
	Name:                   "Hypergolic Pressure-Fed Assembly (HPFA)",
	Description:            "Reliable mid-thrust propulsion utilizing storable bipropellants. Pressure-fed architecture eliminates turbopump complexity for long-term dormancy survival.",
	FlightSlot:             Medium,
	Rarity:                 1.0,
	HealthInitRange:        [2]float64{0.80, 0.95},
	CountRange:             [2]int{1, 2},
	ChamberPressureRange:   [2]float64{10, 50},
	IspVacuumRange:         [2]float64{300, 330},
	IspAtRefPressureRange:  [2]float64{260, 295},
	ReferencePressurePa:    101325,
	ThrustVacuumRange:      [2]float64{5_000, 20_000},
	DryMassRange:           [2]float64{100, 500},
	GimbalEligibleMassKg:   150,
	GimbalRangeRange:       [2]float64{5, 15},
	IgnitionPowerWRange:    [2]float64{0, 0}, // Hypergolic
	OperatingPowerWRange:   [2]float64{20, 120},
	AllowedMixtureIDs:      []string{"MMH_NTO", "Aerozine50_NTO"},
	AllowedCoolingMethods:  []factory.CoolingMethod{factory.Ablative, factory.Radiative},
	AblatorMassKgRange:     [2]float64{5, 50},
	MaxContinuousBurnRange: [2]float64{300, 1000},
	MaxRestarts:            50,
	MinThrottleRange:       [2]float64{0.5, 0.7},
	MaxThrottleRange:       [2]float64{1.0, 1.0},
}

var SCTAMainline = LiquidChemicalArchetype{
	Name:                   "Staged Combustion Turbopump Assembly (SCTA)",
	Description:            "High-performance mainline engine. Utilizes a staged-combustion cycle for maximum specific impulse. High mechanical complexity increases maintenance requirements.",
	FlightSlot:             Medium,
	Rarity:                 0.6,
	HealthInitRange:        [2]float64{0.50, 0.75},
	CountRange:             [2]int{1, 4},
	ChamberPressureRange:   [2]float64{100, 250},
	IspVacuumRange:         [2]float64{420, 460},
	IspAtRefPressureRange:  [2]float64{360, 410},
	ReferencePressurePa:    101325,
	ThrustVacuumRange:      [2]float64{50_000, 500_000},
	DryMassRange:           [2]float64{500, 2000},
	GimbalEligibleMassKg:   500,
	GimbalRangeRange:       [2]float64{5, 12},
	IgnitionPowerWRange:    [2]float64{100, 500},
	OperatingPowerWRange:   [2]float64{50, 200},
	AllowedMixtureIDs:      []string{"LOX_LH2", "Methalox"}, // "Hydrolox" renamed → LOX_LH2
	AllowedCoolingMethods:  []factory.CoolingMethod{factory.Film},
	MaxContinuousBurnRange: [2]float64{100, 400},
	MaxRestarts:            3,
	MinThrottleRange:       [2]float64{0.3, 0.6},
	MaxThrottleRange:       [2]float64{1.0, 1.0},
}

var RDEShockwave = LiquidChemicalArchetype{
	Name:                   "Rotating Detonation Manifold (RDE)",
	Description:            "High-efficiency supersonic detonation ring. Superior thrust-to-weight ratio. Extreme acoustic vibration reduces airframe integrity over long durations.",
	FlightSlot:             Medium,
	Rarity:                 0.35,
	HealthInitRange:        [2]float64{0.40, 0.65},
	CountRange:             [2]int{1, 2},
	ChamberPressureRange:   [2]float64{150, 400},
	IspVacuumRange:         [2]float64{480, 520},
	IspAtRefPressureRange:  [2]float64{400, 450},
	ReferencePressurePa:    101325,
	ThrustVacuumRange:      [2]float64{100_000, 800_000},
	DryMassRange:           [2]float64{600, 2500},
	GimbalEligibleMassKg:   700,
	GimbalRangeRange:       [2]float64{3, 10},
	IgnitionPowerWRange:    [2]float64{200, 800},
	OperatingPowerWRange:   [2]float64{40, 150},
	AllowedMixtureIDs:      []string{"Methane_Fluorine", "Hydrogen_Fluorine"}, // unauthored — warn-and-skip
	AllowedCoolingMethods:  []factory.CoolingMethod{factory.Film, factory.Ablative},
	AblatorMassKgRange:     [2]float64{5, 40},
	MaxContinuousBurnRange: [2]float64{30, 150}, // short — vibration-limited
	MaxRestarts:            10,
	MinThrottleRange:       [2]float64{0.6, 0.8},
	MaxThrottleRange:       [2]float64{1.0, 1.0},
}

var SABRE = LiquidChemicalArchetype{
	Name:                            "Synthetically Actuated Biogenic Reaction Engine (SABRE)",
	Description:                     "A hybrid manifold utilizing a synthetic neural-link to actuate the metabolic exhaust of an engineered extremophile colony. Provides high-efficiency delta-v with a minimal thermal signature. Requires metabolic substrate (CHON) and precise thermal regulation to prevent colony atrophy.",
	FlightSlot:                      Medium,
	Rarity:                          0.15,
	HealthInitRange:                 [2]float64{0.88, 0.98},
	CountRange:                      [2]int{1, 2},
	ChamberPressureRange:            [2]float64{8, 30},
	IspVacuumRange:                  [2]float64{340, 390},
	IspAtRefPressureRange:           [2]float64{280, 340},
	ReferencePressurePa:             101325,
	ThrustVacuumRange:               [2]float64{1_000, 15_000},
	DryMassRange:                    [2]float64{250, 600},
	GimbalEligibleMassKg:            300,
	GimbalRangeRange:                [2]float64{2, 6},
	IgnitionPowerWRange:             [2]float64{10, 50},
	OperatingPowerWRange:            [2]float64{100, 300},
	AllowedMixtureIDs:               []string{"Polyphosphate_Concentrate", "CH3OH_Saline_Substrate"}, // unauthored — warn-and-skip
	AllowedCoolingMethods:           []factory.CoolingMethod{factory.Radiative},
	MaxRestarts:                     -1,
	MaxContinuousBurnRange:          [2]float64{500, 3000},
	MinThrottleRange:                [2]float64{0.4, 0.7},
	MaxThrottleRange:                [2]float64{0.9, 1.0},
	RestartTemperatureCeilingKRange: [2]float64{315, 345},
}

func init() {
	// registerLiquidArchetype validates structurally (panics on failure)
	// and then filters AllowedMixtureIDs to the resolved subset. Archetypes
	// whose mixtures are all unauthored are logged and skipped — that's
	// the mechanism that lets Phase 4 infra land before the user fills
	// in the full mixture catalog.
	registerLiquidArchetype(RCAStandard)
	registerLiquidArchetype(TCAShort)
	registerLiquidArchetype(PDDPhotolytic)
	registerLiquidArchetype(HPFAService)
	registerLiquidArchetype(SCTAMainline)
	registerLiquidArchetype(RDEShockwave)
	registerLiquidArchetype(SABRE)
}
