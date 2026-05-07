package flight

import (
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/google/uuid"
)

// This file owns the liquid-chemical category end-to-end: archetype +
// instance types, validation, the generation DAG, and registration
// plumbing. Archetype *values* live next door in liquid_archetypes.go
// so that file stays a pure content file as more archetypes land.

// ──────────────────────────── Archetype ────────────────────────────────

// LiquidChemicalArchetype is a static, code-defined template for a
// category of liquid-chemical engines. Concrete engines are rolled from
// an archetype by GenerateLiquidChemicalEngine.
//
// Ranges are [lo, hi] pairs. The generation DAG (Plan §2 "Generation
// order — dependency-grouped") defines how dependent samples are drawn
// so no post-hoc clamping is needed.
type LiquidChemicalArchetype struct {
	Name        string
	Description string
	FlightSlot  FlightSlot

	// Rarity is the relative archetype-selection weight inside the slot.
	// 0 is treated as 1.0. A common workhorse is ~1.0; exotic/low-TRL
	// options drop to 0.2–0.4 so they show up, but not routinely.
	Rarity float64

	// ThrustIspBias positions the archetype on the thrust↔Isp axis in
	// [-1, 1]. -1 = punchy (high T/W, low Isp); +1 = efficient (high
	// Isp, lower T/W); 0 = balanced. Read by the civ-aware archetype
	// weighting (commit 5) — civs with a matching ThrustVsIspPreference
	// roll this archetype more often.
	ThrustIspBias float64

	// Group 0 — identity
	HealthInitRange [2]float64
	// CountRange gives the number of identical physical units in the
	// system (redundancy). Inclusive, lo ≥ 1. Each unit gets its own
	// Health entry so damage/repair can act per-unit.
	CountRange [2]int

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
	AllowedMixtures []*factory.Mixture

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
	ChamberPressureBar         float64                  `json:"chamber_pressure_bar"`
	IspVacuumSec               float64                  `json:"isp_vacuum_sec"`
	IspAtRefPressureSec        float64                  `json:"isp_at_ref_pressure_sec"`
	ReferencePressurePa        float64                  `json:"reference_pressure_pa"`
	ThrustVacuumN              float64                  `json:"thrust_vacuum_n"`
	DryMassKg                  float64                  `json:"dry_mass_kg"`
	PropellantConfig           factory.PropellantConfig `json:"propellant_config"`
	IgnitionMethod             factory.IgnitionMethod   `json:"ignition_method"`
	CoolingMethod              factory.CoolingMethod    `json:"cooling_method"`
	Mixture                    *factory.Mixture         `json:"mixture"`
	IgnitionPowerW             float64                  `json:"ignition_power_w"`
	OperatingPowerW            float64                  `json:"operating_power_w"`
	MinThrottle                float64                  `json:"min_throttle"`
	MaxThrottle                float64                  `json:"max_throttle"`
	CanThrottle                bool                     `json:"can_throttle"`
	MaxContinuousBurnSeconds   float64                  `json:"max_continuous_burn_seconds"`
	MaxRestarts                int                      `json:"max_restarts"`
	GimbalRangeDegrees         float64                  `json:"gimbal_range_degrees"`
	CanGimbal                  bool                     `json:"can_gimbal"`
	RestartTemperatureCeilingK float64                  `json:"restart_temperature_ceiling_k"`
	InitialAblatorMassKg       float64                  `json:"initial_ablator_mass_kg"`

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

// IspAt returns effective Isp at the given ambient pressure via the
// two-point linear model from Plan §2. No upper clamp on the ratio —
// dense atmospheres correctly drive Isp to zero (nozzle flow separation).
func (e *LiquidChemicalEngine) IspAt(ambientPa float64) float64 {
	t := ambientPa / e.ReferencePressurePa
	isp := e.IspVacuumSec + t*(e.IspAtRefPressureSec-e.IspVacuumSec)
	if isp < 0 {
		isp = 0
	}
	return isp
}

// HeatToHullW — Phase 4 fills in the real formula. Zero stub so the
// future ship-level thermal bus can call this uniformly on every flight
// system without a type-switch.
func (e *LiquidChemicalEngine) HeatToHullW(throttle float64) float64 { return 0.0 }

// Tick — Phase 4 integrates wear / thermal / ablator-depletion. Panic
// loudly so accidental callers in Phase 3 are obvious.
func (e *LiquidChemicalEngine) Tick(dt, throttle float64) {
	panic("Tick not implemented in Phase 3")
}

// ──────────────────────────── Validation ───────────────────────────────

// registeredArchetypes feeds the package-init Validate() loop. Populated
// by registerLiquidArchetype() calls in liquid_archetypes.go.
var registeredArchetypes []LiquidChemicalArchetype

// RegisterLiquidArchetype enforces structural validation (panics on
// failure) and then filters AllowedMixtures to the subset that
// actually resolves (has Precursors or is Synthetic). Unresolved
// mixtures are logged as warnings — this lets infrastructure land
// before content. An archetype whose mixture list becomes empty after
// filtering is skipped (logged) rather than registered, so the
// generator never picks an unusable archetype. Called from
// factory/content/archetypes_liquid.go at init().
func RegisterLiquidArchetype(a LiquidChemicalArchetype) {
	if err := a.Validate(); err != nil {
		panic(fmt.Sprintf("flight: archetype %q failed validation: %v", a.Name, err))
	}

	resolved := make([]*factory.Mixture, 0, len(a.AllowedMixtures))
	for _, m := range a.AllowedMixtures {
		// Only consider authored if it has Precursors or is explicitly Synthetic.
		if len(m.Precursors) > 0 || m.Synthetic {
			resolved = append(resolved, m)
		} else {
			log.Printf("flight: archetype %q references unauthored mixture %q — dropping reference", a.Name, m.ID)
		}
	}
	if len(resolved) == 0 {
		log.Printf("flight: archetype %q has no authored mixtures — skipping registration", a.Name)
		return
	}
	a.AllowedMixtures = resolved

	registeredArchetypes = append(registeredArchetypes, a)
	coolingNames := make([]string, 0, len(a.AllowedCoolingMethods))
	for _, c := range a.AllowedCoolingMethods {
		coolingNames = append(coolingNames, c.String())
	}
	Register(RegisterOpts{
		Slot: a.FlightSlot,
		Name: a.Name,
		Generator: func(civ *CivBias, rng *rand.Rand) (FlightSystem, error) {
			return GenerateLiquidChemicalEngine(a, civ, rng)
		},
		MinTechTier:        0,
		Rarity:             a.Rarity,
		ThrustIspBias:      a.ThrustIspBias,
		CoolingMethodNames: coolingNames,
	})
}

// Validate checks structural invariants on a single archetype. Plan §2
// "Archetype validation": most single-field range invariants are
// guaranteed structurally by the generation DAG, so we only check what
// the DAG can't.
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

	check(a.CountRange[0] >= 1, fmt.Sprintf("CountRange.lo (%d) must be >= 1", a.CountRange[0]))
	check(a.CountRange[1] >= a.CountRange[0], fmt.Sprintf("CountRange: lo (%d) > hi (%d)", a.CountRange[0], a.CountRange[1]))
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

	// Gimbal gating must not be dead code.
	check(a.GimbalEligibleMassKg >= a.DryMassRange[0],
		"GimbalEligibleMassKg below DryMassRange.lo makes gimbal gating dead code")

	check(len(a.AllowedCoolingMethods) > 0, "AllowedCoolingMethods must be non-empty")
	check(len(a.AllowedMixtures) > 0, "AllowedMixtures must be non-empty")
	// Mixture-resolution is checked at registration time (registerLiquidArchetype)
	// with warn-and-skip semantics so infra can land before content.

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// ──────────────────────────── Generator (DAG) ──────────────────────────

// GenerateLiquidChemicalEngine implements the Plan §2 generation DAG.
// Each group's output conditions subsequent groups so no post-hoc
// clamping is needed.
func GenerateLiquidChemicalEngine(a LiquidChemicalArchetype, civ *CivBias, rng *rand.Rand) (*LiquidChemicalEngine, error) {
	if rng == nil {
		return nil, fmt.Errorf("GenerateLiquidChemicalEngine: rng is nil")
	}
	mfgName, mfgPrefix, tier := manufacturerStamp(civ)

	e := &LiquidChemicalEngine{FlightSlot: a.FlightSlot}

	// ── Group 0 — SystemBase identity ─────────────────────────────────
	e.ID = uuid.New()
	e.ArchetypeName = a.Name
	e.ManufacturerName = mfgName
	e.SerialNumber = factory.PartSerial(mfgPrefix, a.Name, rng)
	e.Name = e.SerialNumber

	// Redundancy: roll a unit count, then roll per-unit initial health so
	// manufacturing variance shows up across siblings in the same system.
	countSpan := a.CountRange[1] - a.CountRange[0] + 1
	e.Count = a.CountRange[0] + rng.Intn(countSpan)
	e.Health = make([]float64, e.Count)
	for i := range e.Health {
		e.Health[i] = rollHealth(a.HealthInitRange, tier, civ, rng)
	}

	// ── Group 1 — ChamberPressureBar (performance driver) ─────────────
	e.ChamberPressureBar = factory.Uniform(a.ChamberPressureRange[0], a.ChamberPressureRange[1], rng)
	pRange := a.ChamberPressureRange[1] - a.ChamberPressureRange[0]
	pNorm := 0.0
	if pRange > 0 {
		pNorm = factory.Clamp01((e.ChamberPressureBar - a.ChamberPressureRange[0]) / pRange)
	}

	// ── Group 2 — Cooling ─────────────────────────────────────────────
	survivors := filterCoolingByPressure(a.AllowedCoolingMethods, e.ChamberPressureBar)
	if len(survivors) == 0 {
		return nil, fmt.Errorf("no cooling methods survive pressure filter for archetype %q at %v bar", a.Name, e.ChamberPressureBar)
	}
	e.CoolingMethod = survivors[rng.Intn(len(survivors))]
	if e.CoolingMethod == factory.Ablative {
		e.InitialAblatorMassKg = factory.Uniform(a.AblatorMassKgRange[0], a.AblatorMassKgRange[1], rng)
	}
	e.AblatorMassRemainingKg = e.InitialAblatorMassKg

	// ── Group 3 — Performance (Isp / Thrust) ──────────────────────────
	// Isp biased toward the upper end proportional to chamber pressure.
	u := rng.Float64()
	bias := u + pNorm*(1-u)
	e.IspVacuumSec = a.IspVacuumRange[0] + bias*(a.IspVacuumRange[1]-a.IspVacuumRange[0])

	// Atmospheric Isp: uniform over [lo, min(hi, IspVacuum)] — guarantees
	// atmospheric ≤ vacuum without a clamp.
	atmHi := a.IspAtRefPressureRange[1]
	if e.IspVacuumSec < atmHi {
		atmHi = e.IspVacuumSec
	}
	e.IspAtRefPressureSec = factory.Uniform(a.IspAtRefPressureRange[0], atmHi, rng)
	e.ReferencePressurePa = a.ReferencePressurePa

	baseThrust := factory.LogUniform(a.ThrustVacuumRange[0], a.ThrustVacuumRange[1], rng)
	thrust := baseThrust * (1 + 0.3*pNorm)
	if thrust > a.ThrustVacuumRange[1] {
		thrust = a.ThrustVacuumRange[1]
	}
	e.ThrustVacuumN = thrust

	// ── Group 4 — DryMass (conditioned on thrust) ─────────────────────
	tRange := a.ThrustVacuumRange[1] - a.ThrustVacuumRange[0]
	tNorm := 0.0
	if tRange > 0 {
		tNorm = factory.Clamp01((e.ThrustVacuumN - a.ThrustVacuumRange[0]) / tRange)
	}
	baseMass := factory.LogUniform(a.DryMassRange[0], a.DryMassRange[1], rng)
	mass := baseMass * (1 + 0.5*tNorm)
	if mass > a.DryMassRange[1] {
		mass = a.DryMassRange[1]
	}
	e.DryMassKg = mass

	// ── Group 5 — Gimbal ──────────────────────────────────────────────
	if e.DryMassKg >= a.GimbalEligibleMassKg {
		e.CanGimbal = true
		e.GimbalRangeDegrees = factory.Uniform(a.GimbalRangeRange[0], a.GimbalRangeRange[1], rng)
	}

	// ── Group 6 — Power ───────────────────────────────────────────────
	e.IgnitionPowerW = factory.Uniform(a.IgnitionPowerWRange[0], a.IgnitionPowerWRange[1], rng)
	e.OperatingPowerW = factory.Uniform(a.OperatingPowerWRange[0], a.OperatingPowerWRange[1], rng)

	// ── Group 7 — Propellant + ignition ───────────────────────────────
	e.Mixture = pickMixture(a.AllowedMixtures, civ, rng)
	mix := e.Mixture
	e.PropellantConfig = mix.Config
	e.IgnitionMethod = deriveIgnition(mix, rng)
	if e.IgnitionMethod == factory.Hypergolic || e.IgnitionMethod == factory.Catalytic {
		e.IgnitionPowerW = 0
	}

	// ── Group 8 — Operational envelope ────────────────────────────────
	burn := factory.Uniform(a.MaxContinuousBurnRange[0], a.MaxContinuousBurnRange[1], rng)
	if e.CoolingMethod == factory.Ablative {
		burn *= 0.5
	}
	e.MaxContinuousBurnSeconds = burn
	e.MaxRestarts = a.MaxRestarts

	e.MinThrottle = factory.Uniform(a.MinThrottleRange[0], a.MinThrottleRange[1], rng)
	e.MaxThrottle = factory.Uniform(a.MaxThrottleRange[0], a.MaxThrottleRange[1], rng)
	if e.MaxThrottle > e.MinThrottle {
		e.CanThrottle = true
	} else {
		e.MinThrottle = 1.0
		e.MaxThrottle = 1.0
		e.CanThrottle = false
	}

	restartBase := factory.Uniform(a.RestartTemperatureCeilingKRange[0], a.RestartTemperatureCeilingKRange[1], rng)
	span := a.RestartTemperatureCeilingKRange[1] - a.RestartTemperatureCeilingKRange[0]
	e.RestartTemperatureCeilingK = restartBase - pNorm*0.2*span
	if e.RestartTemperatureCeilingK < a.RestartTemperatureCeilingKRange[0] {
		e.RestartTemperatureCeilingK = a.RestartTemperatureCeilingKRange[0]
	}

	// ── Group 9 — Runtime state ───────────────────────────────────────
	e.CurrentTemperatureK = 293.15
	return e, nil
}

// ──────────────────────────── DAG helpers ──────────────────────────────

// Pressure thresholds for cooling-method filtering (Plan §2 Group 2).
// Package-level because chamber-pressure cutoffs are physics, not
// per-archetype authoring decisions.
const (
	AblativePressureCeilingBar  = 150.0
	RadiativePressureCeilingBar = 40.0
)

func filterCoolingByPressure(allowed []factory.CoolingMethod, pressureBar float64) []factory.CoolingMethod {
	out := make([]factory.CoolingMethod, 0, len(allowed))
	for _, m := range allowed {
		switch m {
		case factory.Ablative:
			if pressureBar <= AblativePressureCeilingBar {
				out = append(out, m)
			}
		case factory.Radiative:
			if pressureBar <= RadiativePressureCeilingBar {
				out = append(out, m)
			}
		default:
			out = append(out, m)
		}
	}
	return out
}

func deriveIgnition(mix *factory.Mixture, rng *rand.Rand) factory.IgnitionMethod {
	if mix.Hypergolic {
		return factory.Hypergolic
	}
	if mix.Config == factory.Monopropellant {
		return factory.Catalytic
	}
	if rng.Intn(2) == 0 {
		return factory.Spark
	}
	return factory.Pyrotechnic
}

// pickMixture is the civ-aware replacement for the prior uniform
// mixture pick. With civ == nil it collapses to uniform sampling
// (legacy behavior). With civ non-nil, per-mixture weights are:
//
//   - 1.0 baseline;
//   - × 3.0 if PreferredMixtureIDs[m.ID] (Plan §5);
//   - × (1.0 - AversionToCryogenics) if m.Cryogenic;
//   - × 1.5 if PreferredIgnitionTypes[derivedIgnition.String()].
//
// Floor at 0.05 — higher than the archetype-weight floor because there
// are fewer mixtures per archetype, so a near-zero weight is more
// visible. The ignition derivation mirrors deriveIgnition's logic but
// without consuming RNG state — Spark/Pyrotechnic both round-trip
// through the boost as ".Spark" since either is a valid soft match for
// a civ that prefers either.
func pickMixture(allowed []*factory.Mixture, civ *CivBias, rng *rand.Rand) *factory.Mixture {
	if len(allowed) == 0 {
		return nil
	}
	if civ == nil {
		return allowed[rng.Intn(len(allowed))]
	}
	weights := make([]float64, len(allowed))
	total := 0.0
	for i, m := range allowed {
		w := 1.0
		if civ.PreferredMixtureIDs[m.ID] {
			w *= 3.0
		}
		if m.Cryogenic {
			w *= 1.0 - civ.AversionToCryogenics
		}
		if civ.PreferredIgnitionTypes[mixtureIgnitionLabel(m)] {
			w *= 1.5
		}
		if w < 0.05 {
			w = 0.05
		}
		weights[i] = w
		total += w
	}
	r := rng.Float64() * total
	acc := 0.0
	for i, m := range allowed {
		acc += weights[i]
		if r < acc {
			return m
		}
	}
	return allowed[len(allowed)-1]
}

// mixtureIgnitionLabel returns the IgnitionMethod string the mixture
// will resolve to in deriveIgnition. Spark/Pyrotechnic split is
// rng-driven inside the generator; for boosting purposes we project
// both onto "spark" so a civ that prefers either soft-matches. Pure
// label projection — no RNG consumed.
func mixtureIgnitionLabel(m *factory.Mixture) string {
	switch {
	case m.Hypergolic:
		return factory.Hypergolic.String()
	case m.Config == factory.Monopropellant:
		return factory.Catalytic.String()
	default:
		return factory.Spark.String()
	}
}

// rollHealth samples from HealthInitRange, narrowed by TechTier on a
// 1–5 scale (Tier 1 uses the full range; Tier 5 uses only the top
// half), and further shifted by civ.RiskTolerance when civ is non-nil:
// risk=0 → effectiveLo = hi (always tops out); risk=1 → no shift;
// risk=0.5 → midpoint. The risk shift composes on top of tier
// narrowing — the picker uses whichever lower bound is *higher*, so a
// conservative civ at low TechTier still rolls a healthy part.
func rollHealth(hr [2]float64, tier int, civ *CivBias, rng *rand.Rand) float64 {
	if tier < 1 {
		tier = 1
	}
	if tier > 5 {
		tier = 5
	}
	lo, hi := hr[0], hr[1]
	span := hi - lo
	tierLo := lo + float64(tier-1)/4.0*(span*0.5)
	effectiveLo := tierLo
	if civ != nil {
		riskLo := lo + span*(1.0-civ.RiskTolerance)
		if riskLo > effectiveLo {
			effectiveLo = riskLo
		}
	}
	if effectiveLo > hi {
		effectiveLo = hi
	}
	return factory.Uniform(effectiveLo, hi, rng)
}
