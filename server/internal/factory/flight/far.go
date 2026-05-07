package flight

import (
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/google/uuid"
)

// This file owns the Far (relativistic) flight category. Field set
// diverges from LiquidChemicalArchetype: no ChamberPressure, no cooling
// method taxonomy, no per-unit Count (a Far drive is a single coherent
// reactor, not a cluster). What matters instead is TopSpeedFractionC
// (the primary gameplay dial), continuous OperatingPower (containment
// dominates), and an extreme specific impulse range. See Phase 4 Plan §1.

// ──────────────────────────── Archetype ────────────────────────────────

type RelativisticDriveArchetype struct {
	Name        string
	Description string
	FlightSlot  FlightSlot // always Far

	// Tier gating. Phase 4 has only RBCA at TechTier 5.
	TechTier int

	// Rarity is the relative archetype-selection weight inside the slot.
	// 0 is treated as 1.0. See LiquidChemicalArchetype.Rarity.
	Rarity float64

	// ThrustIspBias positions the archetype on the thrust↔Isp axis in
	// [-1, 1]. See LiquidChemicalArchetype.ThrustIspBias.
	ThrustIspBias float64

	HealthInitRange [2]float64

	// TopSpeedFractionC is the coordinate-frame top speed as a fraction
	// of c. Primary gameplay dial — controls transit time and Lorentz γ.
	TopSpeedFractionC float64

	// Extreme ranges for relativistic drives: millions of seconds of Isp,
	// hundreds to thousands of newtons of continuous push.
	IspVacuumRange       [2]float64
	ThrustNRange         [2]float64
	OperatingPowerWRange [2]float64

	AllowedMixtures []*factory.Mixture

	// SignatureProfile is flavor text in Phase 4. The scanner system
	// (future phase) restructures this into a spectrum/intensity schema.
	SignatureProfile string
}

// ──────────────────────────── Instance ─────────────────────────────────

type RelativisticDrive struct {
	factory.SystemBase
	FlightSlot FlightSlot `json:"flight_slot"`

	TechTier          int     `json:"tech_tier"`
	TopSpeedFractionC float64 `json:"top_speed_fraction_c"`
	IspVacuumSec      float64 `json:"isp_vacuum_sec"`
	ThrustN           float64 `json:"thrust_n"`
	OperatingPowerW   float64 `json:"operating_power_w"`
	Mixture           *factory.Mixture `json:"mixture"`
	SignatureProfile  string  `json:"signature_profile"`
}

// ──────────────────────────── Validation ───────────────────────────────

var registeredRelativisticArchetypes []RelativisticDriveArchetype

func (a RelativisticDriveArchetype) Validate() error {
	var errs []error
	check := func(cond bool, msg string) {
		if !cond {
			errs = append(errs, errors.New(msg))
		}
	}
	checkRange := func(name string, r [2]float64) {
		check(r[0] <= r[1], fmt.Sprintf("%s: lo (%v) > hi (%v)", name, r[0], r[1]))
	}

	check(a.Name != "", "Name must not be empty")
	check(a.FlightSlot == Far, "RelativisticDriveArchetype.FlightSlot must be Far")
	check(a.TechTier >= 1 && a.TechTier <= 5, fmt.Sprintf("TechTier %d outside [1,5]", a.TechTier))
	check(a.TopSpeedFractionC > 0 && a.TopSpeedFractionC < 1, fmt.Sprintf("TopSpeedFractionC %v must be in (0,1)", a.TopSpeedFractionC))

	checkRange("HealthInitRange", a.HealthInitRange)
	check(a.HealthInitRange[0] >= 0 && a.HealthInitRange[1] <= 1, "HealthInitRange must be ⊂ [0,1]")
	checkRange("IspVacuumRange", a.IspVacuumRange)
	checkRange("ThrustNRange", a.ThrustNRange)
	checkRange("OperatingPowerWRange", a.OperatingPowerWRange)

	check(len(a.AllowedMixtures) > 0, "AllowedMixtures must be non-empty")
	check(a.SignatureProfile != "", "SignatureProfile must be non-empty (flavor stub)")

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// RegisterRelativisticArchetype mirrors RegisterLiquidArchetype:
// structural validation panics; mixture resolution warn-and-skips.
// Called from factory/content/archetypes_far.go at init().
func RegisterRelativisticArchetype(a RelativisticDriveArchetype) {
	if err := a.Validate(); err != nil {
		panic(fmt.Sprintf("flight: Far archetype %q failed validation: %v", a.Name, err))
	}

	resolved := make([]*factory.Mixture, 0, len(a.AllowedMixtures))
	for _, m := range a.AllowedMixtures {
		if len(m.Precursors) > 0 || m.Synthetic {
			resolved = append(resolved, m)
		} else {
			log.Printf("flight: Far archetype %q references unauthored mixture %q — dropping reference", a.Name, m.ID)
		}
	}
	if len(resolved) == 0 {
		log.Printf("flight: Far archetype %q has no authored mixtures — skipping registration", a.Name)
		return
	}
	a.AllowedMixtures = resolved

	registeredRelativisticArchetypes = append(registeredRelativisticArchetypes, a)
	Register(RegisterOpts{
		Slot: a.FlightSlot,
		Name: a.Name,
		Generator: func(civ *CivBias, rng *rand.Rand) (FlightSystem, error) {
			return GenerateRelativisticDrive(a, civ, rng)
		},
		MinTechTier:   a.TechTier,
		Rarity:        a.Rarity,
		ThrustIspBias: a.ThrustIspBias,
	})
}

// ──────────────────────────── Generator ────────────────────────────────

func GenerateRelativisticDrive(a RelativisticDriveArchetype, civ *CivBias, rng *rand.Rand) (*RelativisticDrive, error) {
	if rng == nil {
		return nil, fmt.Errorf("GenerateRelativisticDrive: rng is nil")
	}
	mfgName, mfgPrefix, tier := manufacturerStamp(civ)

	d := &RelativisticDrive{
		FlightSlot:        a.FlightSlot,
		TechTier:          a.TechTier,
		TopSpeedFractionC: a.TopSpeedFractionC,
		SignatureProfile:  a.SignatureProfile,
	}
	d.ID = uuid.New()
	d.ArchetypeName = a.Name
	d.ManufacturerName = mfgName
	d.SerialNumber = factory.PartSerial(mfgPrefix, a.Name, rng)
	d.Name = d.SerialNumber

	// A Far drive is one coherent reactor — single unit, single health.
	d.Count = 1
	d.Health = []float64{rollHealth(a.HealthInitRange, tier, civ, rng)}

	d.IspVacuumSec = factory.LogUniform(a.IspVacuumRange[0], a.IspVacuumRange[1], rng)
	d.ThrustN = factory.LogUniform(a.ThrustNRange[0], a.ThrustNRange[1], rng)
	d.OperatingPowerW = factory.LogUniform(a.OperatingPowerWRange[0], a.OperatingPowerWRange[1], rng)

	d.Mixture = pickMixture(a.AllowedMixtures, civ, rng)

	return d, nil
}
