// Package flight hosts flight-system archetypes, instances, and the
// slot dispatcher. Every finished ship has three flight slots
// (Short / Medium / Far).
package flight

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
)

// FlightSlot identifies which of the three mandatory flight-slot roles
// an archetype fills.
type FlightSlot int

const (
	Short FlightSlot = iota
	Medium
	Far
)

func (s FlightSlot) String() string {
	switch s {
	case Short:
		return "short"
	case Medium:
		return "medium"
	case Far:
		return "far"
	}
	return fmt.Sprintf("flight_slot(%d)", int(s))
}

func (s FlightSlot) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

func (s *FlightSlot) UnmarshalText(text []byte) error {
	switch string(text) {
	case "short":
		*s = Short
	case "medium":
		*s = Medium
	case "far":
		*s = Far
	default:
		return fmt.Errorf("unknown flight slot %q", string(text))
	}
	return nil
}

// FlightSystem is the interface every flight-slot instance satisfies.
// Deliberately empty — grows methods once a second flight category
// exists and there's something worth abstracting.
type FlightSystem interface{}

// ErrSlotEmpty means no archetype is registered for the requested slot
// (or none survive tier filtering). Returned to assembly as the
// "leave this slot null" signal.
var ErrSlotEmpty = errors.New("flight: no archetype registered for slot")

// archetypeGenerator produces a concrete FlightSystem. The dispatcher
// owns archetype selection then hands the chosen civ + rng to this
// function. civ is nil when the caller doesn't supply one (legacy/test
// path); the generator falls back to tier-3 / no-bias defaults.
type archetypeGenerator func(civ *CivBias, rng *rand.Rand) (FlightSystem, error)

type archetypeEntry struct {
	archetypeName string
	generate      archetypeGenerator
	minTechTier   int     // 0 = no gate; civ must satisfy civTier >= minTechTier
	rarity        float64 // > 0 relative weight; 0 is treated as 1.0 at sample time
	// thrustIspBias positions the archetype on the thrust↔Isp axis,
	// range [-1, 1]. -1 = punchy (high T/W, low Isp); +1 = efficient
	// (high Isp, lower T/W). Range is enforced at registration.
	thrustIspBias float64
	// coolingNames is the archetype's AllowedCoolingMethods rendered as
	// String() values, captured at registration so the dispatcher can
	// check overlap against CivBias.PreferredCoolingMethods. Empty
	// means "no cooling concept" (e.g. relativistic drives).
	coolingNames []string
}

// slotRegistry maps each flight slot to its registered archetypes.
// Populated via Register() from per-category init() funcs.
var slotRegistry = map[FlightSlot][]archetypeEntry{}

// RegisterOpts is the options bag for registering a new archetype with
// the dispatcher.
type RegisterOpts struct {
	Slot               FlightSlot
	Name               string
	Generator          func(civ *CivBias, rng *rand.Rand) (FlightSystem, error)
	MinTechTier        int
	Rarity             float64
	ThrustIspBias      float64
	CoolingMethodNames []string
}

// Register adds an archetype to the slot dispatcher. Called from
// per-category content files (factory/content/archetypes_*.go) at
// init(). Validates ThrustIspBias range; panics on misauth so
// authoring mistakes surface at startup.
func Register(o RegisterOpts) {
	if o.ThrustIspBias < -1 || o.ThrustIspBias > 1 {
		panic(fmt.Sprintf("flight: archetype %q ThrustIspBias %v out of [-1, 1]", o.Name, o.ThrustIspBias))
	}
	slotRegistry[o.Slot] = append(slotRegistry[o.Slot], archetypeEntry{
		archetypeName: o.Name,
		generate:      o.Generator,
		minTechTier:   o.MinTechTier,
		rarity:        o.Rarity,
		thrustIspBias: o.ThrustIspBias,
		coolingNames:  o.CoolingMethodNames,
	})
}

// archetypeFavoursCooling reports whether any of the archetype's
// allowed cooling methods (captured at registration as String()s)
// match the civ's preferred cooling methods.
func archetypeFavoursCooling(e archetypeEntry, prefs map[string]bool) bool {
	if len(prefs) == 0 || len(e.coolingNames) == 0 {
		return false
	}
	for _, c := range e.coolingNames {
		if prefs[c] {
			return true
		}
	}
	return false
}

// defaultTechTier is the tier the dispatcher uses when civ is nil —
// matches the historical GenericCivilization fallback.
const defaultTechTier = 3

// GenerateForSlot picks an archetype registered for the slot (filtered
// by the civ's TechTier against each archetype's minTechTier) and runs
// the chosen archetype's generator. The civ-aware rarity weighting
// (ThrustVsIspPreference proximity, RiskTolerance sharpening,
// PreferredCoolingMethods overlap) applies when civ is non-nil.
func GenerateForSlot(slot FlightSlot, civ *CivBias, rng *rand.Rand) (FlightSystem, error) {
	entries := slotRegistry[slot]
	if len(entries) == 0 {
		return nil, ErrSlotEmpty
	}

	tier := defaultTechTier
	if civ != nil {
		tier = civ.TechTier
	}
	eligible := make([]archetypeEntry, 0, len(entries))
	for _, e := range entries {
		if e.minTechTier <= tier {
			eligible = append(eligible, e)
		}
	}
	if len(eligible) == 0 {
		return nil, ErrSlotEmpty
	}

	// Weighted archetype pick. rarity 0 → treat as 1.0. When civ is
	// non-nil, the rarity weight is multiplied by:
	//   - ThrustVsIspPreference proximity to e.thrustIspBias (× up to 1.8);
	//   - RiskTolerance rarity-sharpening exponent (low risk sharpens
	//     toward the workhorse, high risk flattens the distribution);
	//   - PreferredCoolingMethods overlap with the archetype's allowed
	//     cooling methods (× 1.5 if any overlap).
	// All multiplicative — order doesn't matter. Floor at 0.01 so an
	// extreme civ never zeros out an eligible archetype entirely.
	weights := make([]float64, len(eligible))
	total := 0.0
	for i, e := range eligible {
		w := e.rarity
		if w <= 0 {
			w = 1.0
		}
		if civ != nil {
			diff := math.Abs(civ.ThrustVsIspPreference - e.thrustIspBias)
			w *= 1.0 + 0.8*(1.0-diff/2.0)
			// risk=0.0 → exp=2 (sharpen toward higher rarity weights);
			// risk=0.5 → exp=1 (no-op); risk=1.0 → exp=0 (flatten to
			// near-uniform). Clamp to [0.1, 3] to avoid pathological
			// distributions.
			exp := math.Max(0.1, math.Min(3.0, 1.0+(1.0-2.0*civ.RiskTolerance)))
			w = math.Pow(w, exp)
			if archetypeFavoursCooling(e, civ.PreferredCoolingMethods) {
				w *= 1.5
			}
		}
		if w < 0.01 {
			w = 0.01
		}
		weights[i] = w
		total += w
	}
	r := rng.Float64() * total
	acc := 0.0
	arch := eligible[len(eligible)-1]
	for i, e := range eligible {
		acc += weights[i]
		if r < acc {
			arch = e
			break
		}
	}

	return arch.generate(civ, rng)
}
