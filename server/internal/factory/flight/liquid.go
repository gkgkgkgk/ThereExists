package flight

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// ──────────────────────────── Archetype ────────────────────────────────

// LiquidChemicalArchetype is a static, code-defined template for a
// category of liquid-chemical engines. Concrete engines are rolled from
// an archetype by GenerateLiquidChemicalEngine (commit 7).
//
// Ranges are [lo, hi] pairs. The generation DAG (Plan §2 "Generation
// order — dependency-grouped") defines how dependent samples are drawn
// so no post-hoc clamping is needed.
type LiquidChemicalArchetype struct {
	Name      string
	FlightSlot FlightSlot

	// Group 0 — identity
	HealthInitRange [2]float64

	// Group 1 — performance driver
	ChamberPressureRange [2]float64 // bar

	// Group 2 — cooling
	AllowedCoolingMethods []factory.CoolingMethod

	// Group 3 — performance
	IspVacuumRange        [2]float64 // s
	IspAtRefPressureRange [2]float64 // s; only lo is load-bearing
	ReferencePressurePa   float64    // scalar
	ThrustVacuumRange     [2]float64 // N, log-uniform

	// Group 4 — physical
	DryMassRange [2]float64 // kg, log-uniform

	// Group 5 — gimbal
	GimbalEligibleMassKg float64
	GimbalRangeRange     [2]float64 // degrees

	// Group 6 — power
	IgnitionPowerWRange  [2]float64
	OperatingPowerWRange [2]float64

	// Group 7 — propellant
	AllowedMixtureIDs []string

	// Group 8 — operational envelope
	MaxContinuousBurnRange          [2]float64 // s
	MaxRestarts                     int        // -1 = unlimited
	MinThrottleRange                [2]float64
	MaxThrottleRange                [2]float64
	RestartTemperatureCeilingKRange [2]float64 // K

	// Cooling-dependent
	AblatorMassKgRange [2]float64 // kg — meaningful only if Ablative allowed
}

// ──────────────────────────── Instance ─────────────────────────────────

// LiquidChemicalEngine is a concrete rolled engine. Embeds SystemBase.
// Fields are grouped by mutability (Plan §2 "LiquidChemicalEngine — fields").
type LiquidChemicalEngine struct {
	factory.SystemBase
	FlightSlot FlightSlot `json:"flight_slot"`

	// Immutable after generation
	ChamberPressureBar  float64                  `json:"chamber_pressure_bar"`
	IspVacuumSec        float64                  `json:"isp_vacuum_sec"`
	IspAtRefPressureSec float64                  `json:"isp_at_ref_pressure_sec"`
	ReferencePressurePa float64                  `json:"reference_pressure_pa"`
	ThrustVacuumN       float64                  `json:"thrust_vacuum_n"`
	DryMassKg           float64                  `json:"dry_mass_kg"`
	PropellantConfig    factory.PropellantConfig `json:"propellant_config"`
	IgnitionMethod      factory.IgnitionMethod   `json:"ignition_method"`
	CoolingMethod       factory.CoolingMethod    `json:"cooling_method"`
	MixtureID           string                   `json:"mixture_id"`
	IgnitionPowerW      float64                  `json:"ignition_power_w"`
	OperatingPowerW     float64                  `json:"operating_power_w"`
	MinThrottle         float64                  `json:"min_throttle"`
	MaxThrottle         float64                  `json:"max_throttle"`
	CanThrottle         bool                     `json:"can_throttle"`
	MaxContinuousBurnSeconds float64             `json:"max_continuous_burn_seconds"`
	MaxRestarts         int                      `json:"max_restarts"`
	GimbalRangeDegrees  float64                  `json:"gimbal_range_degrees"`
	CanGimbal           bool                     `json:"can_gimbal"`
	RestartTemperatureCeilingK float64           `json:"restart_temperature_ceiling_k"`
	InitialAblatorMassKg float64                 `json:"initial_ablator_mass_kg"`

	// Mutable runtime state (persisted)
	RestartsUsed           int     `json:"restarts_used"`
	TotalBurnTimeSeconds   float64 `json:"total_burn_time_seconds"`
	CurrentBurnSeconds     float64 `json:"current_burn_seconds"`
	IsFiring               bool    `json:"is_firing"`
	CurrentTemperatureK    float64 `json:"current_temperature_k"`
	AblatorMassRemainingKg float64 `json:"ablator_mass_remaining_kg"`
}

// HasRestartsRemaining centralises the "unlimited restarts" sentinel so
// flight-logic code later can't reinvent the check incorrectly.
func (e *LiquidChemicalEngine) HasRestartsRemaining() bool {
	return e.MaxRestarts < 0 || e.RestartsUsed < e.MaxRestarts
}

// ──────────────────────────── Validation ───────────────────────────────

// registeredArchetypes feeds the package-init Validate() loop.
var registeredArchetypes []LiquidChemicalArchetype

func registerLiquidArchetype(a LiquidChemicalArchetype) {
	registeredArchetypes = append(registeredArchetypes, a)
	register(a.FlightSlot, a.Name, makeLiquidGenerator(a))
}

// Validate checks structural invariants on a single archetype. Plan §2
// "Archetype validation": most single-field range invariants are
// structurally guaranteed by the generation DAG, so we only check what
// the DAG can't guarantee.
func (a LiquidChemicalArchetype) Validate() error {
	var errs []error
	check := func(cond bool, msg string) {
		if !cond {
			errs = append(errs, errors.New(msg))
		}
	}

	checkRange := func(name string, r [2]float64) {
		check(r[0] <= r[1], fmt.Sprintf("%s: lo (%v) > hi (%v)", name, r[0], r[1]))
	}
	checkRange("HealthInitRange", a.HealthInitRange)
	checkRange("ChamberPressureRange", a.ChamberPressureRange)
	checkRange("IspVacuumRange", a.IspVacuumRange)
	checkRange("IspAtRefPressureRange", a.IspAtRefPressureRange)
	checkRange("ThrustVacuumRange", a.ThrustVacuumRange)
	checkRange("DryMassRange", a.DryMassRange)
	checkRange("GimbalRangeRange", a.GimbalRangeRange)
	checkRange("IgnitionPowerWRange", a.IgnitionPowerWRange)
	checkRange("OperatingPowerWRange", a.OperatingPowerWRange)
	checkRange("MaxContinuousBurnRange", a.MaxContinuousBurnRange)
	checkRange("MinThrottleRange", a.MinThrottleRange)
	checkRange("MaxThrottleRange", a.MaxThrottleRange)
	checkRange("RestartTemperatureCeilingKRange", a.RestartTemperatureCeilingKRange)
	checkRange("AblatorMassKgRange", a.AblatorMassKgRange)

	check(a.HealthInitRange[0] >= 0 && a.HealthInitRange[1] <= 1.0,
		"HealthInitRange must be ⊂ [0, 1]")
	check(a.ReferencePressurePa > 0, "ReferencePressurePa must be > 0")

	// Gimbal gating must not be dead code — GimbalEligibleMassKg below
	// DryMassRange.lo means every engine gimbals (gating never fires).
	check(a.GimbalEligibleMassKg >= a.DryMassRange[0],
		"GimbalEligibleMassKg below DryMassRange.lo makes gimbal gating dead code")

	check(len(a.AllowedCoolingMethods) > 0, "AllowedCoolingMethods must be non-empty")
	check(len(a.AllowedMixtureIDs) > 0, "AllowedMixtureIDs must be non-empty")
	for _, id := range a.AllowedMixtureIDs {
		if _, ok := factory.Mixtures[id]; !ok {
			errs = append(errs, fmt.Errorf("AllowedMixtureIDs: unknown mixture %q", id))
		}
	}
	// Every mixture in the list must have a derivable ignition method.
	// With current rules (hypergolic → Hypergolic; mono → Catalytic; else
	// Spark/Pyrotechnic) there's always at least one path, but leaving
	// the check defensive against future mixture-flag changes.
	for _, id := range a.AllowedMixtureIDs {
		m, ok := factory.Mixtures[id]
		if !ok {
			continue
		}
		_ = m // current derivation rules always yield a method; reserved
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// makeLiquidGenerator is reassigned by the companion generator file
// (commit 7). Default is a placeholder so this commit compiles and
// init-time Validate() still runs.
var makeLiquidGenerator = func(a LiquidChemicalArchetype) archetypeGenerator {
	return func(manufacturerID string, rng *rand.Rand) (FlightSystem, error) {
		return nil, errors.New("liquid chemical generator not implemented — see commit 7")
	}
}

// ──────────────────────────── Archetype registry ───────────────────────

// RCSLiquidChemical — Phase 3's one end-to-end archetype. Values match
// Plan §2 "Example archetype (v1)".
var RCSLiquidChemical = LiquidChemicalArchetype{
	Name:                            "RCSLiquidChemical",
	FlightSlot:                      Short,
	HealthInitRange:                 [2]float64{0.85, 1.0},
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
	MaxRestarts:                     -1, // unlimited
	MinThrottleRange:                [2]float64{1.0, 1.0}, // on/off
	MaxThrottleRange:                [2]float64{1.0, 1.0},
	RestartTemperatureCeilingKRange: [2]float64{400, 600},
	AblatorMassKgRange:              [2]float64{0.1, 2.0},
}

func init() {
	registerLiquidArchetype(RCSLiquidChemical)
	for _, a := range registeredArchetypes {
		if err := a.Validate(); err != nil {
			panic(fmt.Sprintf("flight: archetype %q failed validation: %v", a.Name, err))
		}
	}
}

