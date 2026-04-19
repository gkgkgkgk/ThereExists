package flight

import (
	"fmt"
	"math/rand"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/google/uuid"
)

// GenerateLiquidChemicalEngine implements the Plan §2 generation DAG.
// Each group's output conditions subsequent groups so no post-hoc
// clamping is needed.
func GenerateLiquidChemicalEngine(a LiquidChemicalArchetype, ctx factory.GenContext) (*LiquidChemicalEngine, error) {
	rng := ctx.Rng
	if rng == nil {
		return nil, fmt.Errorf("GenerateLiquidChemicalEngine: ctx.Rng is nil")
	}
	mfg, ok := factory.Manufacturers[ctx.ManufacturerID]
	if !ok {
		return nil, fmt.Errorf("GenerateLiquidChemicalEngine: unknown manufacturer %q", ctx.ManufacturerID)
	}
	civ, ok := factory.Civilizations[mfg.CivilizationID]
	if !ok {
		return nil, fmt.Errorf("GenerateLiquidChemicalEngine: unknown civilization %q for manufacturer %q", mfg.CivilizationID, mfg.ID)
	}

	e := &LiquidChemicalEngine{
		FlightSlot: a.FlightSlot,
	}

	// ── Group 0 — SystemBase identity ─────────────────────────────────
	e.ID = uuid.New()
	e.ArchetypeName = a.Name
	e.ManufacturerID = mfg.ID
	e.SerialNumber = mfg.NamingConvention(rng, a.Name)
	e.Name = e.SerialNumber
	e.Health = rollHealth(a.HealthInitRange, civ.TechTier, rng)

	// ── Group 1 — ChamberPressureBar (performance driver) ─────────────
	e.ChamberPressureBar = factory.Uniform(a.ChamberPressureRange[0], a.ChamberPressureRange[1], rng)

	// pNorm: [0,1] along the archetype's pressure band, for downstream bias.
	pRange := a.ChamberPressureRange[1] - a.ChamberPressureRange[0]
	pNorm := 0.0
	if pRange > 0 {
		pNorm = factory.Clamp01((e.ChamberPressureBar - a.ChamberPressureRange[0]) / pRange)
	}

	// ── Group 2 — Cooling ─────────────────────────────────────────────
	survivors := filterCoolingByPressure(a.AllowedCoolingMethods, e.ChamberPressureBar)
	if len(survivors) == 0 {
		// Archetype validation should prevent this, but bail loudly
		// instead of panicking deep inside the DAG.
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

	// Atmospheric Isp: uniform over [lo, min(hi, IspVacuum)]. Guarantees
	// IspAtRef ≤ IspVacuum without a clamp.
	atmHi := a.IspAtRefPressureRange[1]
	if e.IspVacuumSec < atmHi {
		atmHi = e.IspVacuumSec
	}
	e.IspAtRefPressureSec = factory.Uniform(a.IspAtRefPressureRange[0], atmHi, rng)

	e.ReferencePressurePa = a.ReferencePressurePa

	// Thrust log-uniform, mildly biased upward by pressure.
	baseThrust := factory.LogUniform(a.ThrustVacuumRange[0], a.ThrustVacuumRange[1], rng)
	thrust := baseThrust * (1 + 0.3*pNorm)
	if thrust > a.ThrustVacuumRange[1] {
		thrust = a.ThrustVacuumRange[1]
	}
	e.ThrustVacuumN = thrust

	// ── Group 4 — Physical (dry mass, conditioned on thrust) ──────────
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

	// ── Group 5 — Gimbal (depends on dry mass) ────────────────────────
	if e.DryMassKg >= a.GimbalEligibleMassKg {
		e.CanGimbal = true
		e.GimbalRangeDegrees = factory.Uniform(a.GimbalRangeRange[0], a.GimbalRangeRange[1], rng)
	}

	// ── Group 6 — Power ───────────────────────────────────────────────
	e.IgnitionPowerW = factory.Uniform(a.IgnitionPowerWRange[0], a.IgnitionPowerWRange[1], rng)
	e.OperatingPowerW = factory.Uniform(a.OperatingPowerWRange[0], a.OperatingPowerWRange[1], rng)

	// ── Group 7 — Propellant + ignition ───────────────────────────────
	e.MixtureID = a.AllowedMixtureIDs[rng.Intn(len(a.AllowedMixtureIDs))]
	mix, ok := factory.Mixtures[e.MixtureID]
	if !ok {
		return nil, fmt.Errorf("unknown mixture %q for archetype %q", e.MixtureID, a.Name)
	}
	e.PropellantConfig = mix.Config
	e.IgnitionMethod = deriveIgnition(mix, rng)

	// Zero ignition power for mixtures that don't need an ignition circuit.
	if e.IgnitionMethod == factory.Hypergolic || e.IgnitionMethod == factory.Catalytic {
		e.IgnitionPowerW = 0
	}

	// ── Group 8 — Operational envelope ────────────────────────────────
	burn := factory.Uniform(a.MaxContinuousBurnRange[0], a.MaxContinuousBurnRange[1], rng)
	if e.CoolingMethod == factory.Ablative {
		burn *= 0.5 // Ablators can't sustain long burns.
	}
	e.MaxContinuousBurnSeconds = burn
	e.MaxRestarts = a.MaxRestarts

	e.MinThrottle = factory.Uniform(a.MinThrottleRange[0], a.MinThrottleRange[1], rng)
	e.MaxThrottle = factory.Uniform(a.MaxThrottleRange[0], a.MaxThrottleRange[1], rng)
	if e.MaxThrottle > e.MinThrottle {
		e.CanThrottle = true
	} else {
		// Collapse to fixed 1.0 for on/off engines so downstream code
		// doesn't read two slightly-different throttles.
		e.MinThrottle = 1.0
		e.MaxThrottle = 1.0
		e.CanThrottle = false
	}

	// Restart ceiling: higher chamber pressure → tighter ceiling.
	restartBase := factory.Uniform(a.RestartTemperatureCeilingKRange[0], a.RestartTemperatureCeilingKRange[1], rng)
	restartRangeSpan := a.RestartTemperatureCeilingKRange[1] - a.RestartTemperatureCeilingKRange[0]
	e.RestartTemperatureCeilingK = restartBase - pNorm*0.2*restartRangeSpan
	if e.RestartTemperatureCeilingK < a.RestartTemperatureCeilingKRange[0] {
		e.RestartTemperatureCeilingK = a.RestartTemperatureCeilingKRange[0]
	}

	// ── Group 9 — Runtime state ───────────────────────────────────────
	e.RestartsUsed = 0
	e.TotalBurnTimeSeconds = 0
	e.CurrentBurnSeconds = 0
	e.IsFiring = false
	e.CurrentTemperatureK = 293.15 // 20 °C ambient; ship-level gen may override later.

	return e, nil
}

// ──────────────────────────── Helpers ──────────────────────────────────

// Package-level pressure thresholds for cooling-method filtering.
// Plan §2 Group 2. These live here rather than on the archetype so every
// archetype uses the same physics — chamber-pressure cutoffs are not a
// per-archetype authoring decision.
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
	// Bipropellant, non-hypergolic — pick between Spark and Pyrotechnic.
	if rng.Intn(2) == 0 {
		return factory.Spark
	}
	return factory.Pyrotechnic
}

// rollHealth samples from the archetype's HealthInitRange, narrowed by
// the civilization's TechTier on a 1–5 scale. Tier 1 gets the full
// range; Tier 5 gets only the top half. Monotonic in tier.
func rollHealth(hr [2]float64, tier int, rng *rand.Rand) float64 {
	if tier < 1 {
		tier = 1
	}
	if tier > 5 {
		tier = 5
	}
	lo, hi := hr[0], hr[1]
	span := hi - lo
	narrowedLo := lo + float64(tier-1)/4.0*(span*0.5)
	return factory.Uniform(narrowedLo, hi, rng)
}

// ──────────────────────────── Instance methods ─────────────────────────

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

// ──────────────────────────── Wiring ───────────────────────────────────

func init() {
	// Replace the placeholder from liquid.go with the real generator.
	makeLiquidGenerator = func(a LiquidChemicalArchetype) archetypeGenerator {
		return func(manufacturerID string, rng *rand.Rand) (FlightSystem, error) {
			return GenerateLiquidChemicalEngine(a, factory.GenContext{
				ManufacturerID: manufacturerID,
				Rng:            rng,
			})
		}
	}

	// Re-register archetypes against the real generator. registerLiquid-
	// Archetype captured the placeholder at init-time in liquid.go, so
	// we rebuild the slotRegistry entries with the real generator here.
	rebuildSlotRegistry()
}

func rebuildSlotRegistry() {
	// Nuke entries for every slot that has any registered liquid archetype.
	affected := map[FlightSlot]struct{}{}
	for _, a := range registeredArchetypes {
		affected[a.FlightSlot] = struct{}{}
	}
	for slot := range affected {
		delete(slotRegistry, slot)
	}
	// Re-register with the real generator.
	for _, a := range registeredArchetypes {
		register(a.FlightSlot, a.Name, makeLiquidGenerator(a))
	}
}
