// Package flight hosts flight-system archetypes, instances, and the
// slot dispatcher. Every finished ship has three flight slots
// (Short / Medium / Far). Phase 3 populates only Short.
package flight

import (
	"errors"
	"fmt"
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
// Deliberately empty in Phase 3 — grows methods once a second flight
// category exists and there's something worth abstracting.
//nolint:golint // empty interface is intentional — grows methods in Phase 4.
type FlightSystem interface{}

// ErrSlotEmpty means no archetype is registered for the requested slot.
// Phase 3 returns this for Medium and Far.
var ErrSlotEmpty = errors.New("flight: no archetype registered for slot")

// archetypeGenerator produces a concrete FlightSystem for a chosen
// manufacturer. The dispatcher owns archetype + manufacturer selection,
// then hands both to this function.
type archetypeGenerator func(manufacturerID string, rng *rand.Rand) (FlightSystem, error)

type archetypeEntry struct {
	archetypeName string
	generate      archetypeGenerator
	minTechTier   int     // 0 = no gate; civ must satisfy civTier >= minTechTier
	rarity        float64 // > 0 relative weight; 0 is treated as 1.0 at sample time
}

// slotRegistry maps each flight slot to its registered archetypes.
// Populated via register() from per-category init() funcs (e.g. liquid.go).
var slotRegistry = map[FlightSlot][]archetypeEntry{}

// registerFull registers an archetype with the slot dispatcher.
// minTechTier 0 means no gate; otherwise civs with TechTier <
// minTechTier never see this archetype and their slot rolls as
// ErrSlotEmpty if nothing else survives filtering. rarity is a
// relative weight used by GenerateForSlot's weighted archetype pick; 0
// is treated as 1.0 at sample time. A rarity of 0.2 means the
// archetype is picked ~5× less often than a rarity-1.0 sibling.
func registerFull(slot FlightSlot, name string, gen archetypeGenerator, minTechTier int, rarity float64) {
	slotRegistry[slot] = append(slotRegistry[slot], archetypeEntry{
		archetypeName: name,
		generate:      gen,
		minTechTier:   minTechTier,
		rarity:        rarity,
	})
}

// ManufacturerPicker resolves a manufacturer for a given civ + archetype.
// previousManufacturerID is a provenance hint: when non-empty, pickers
// that honour it (e.g. factory.PickManufacturerBiased) bias toward the
// same manufacturer as the previous slot so a ship tends to ship as a
// coherent kit rather than a yard-sale of vendors. Pass "" for the first
// slot, or from any picker that ignores the hint.
//
// Injected so the flight package doesn't import the factory root, which
// would close an import cycle.
type ManufacturerPicker func(civilizationID, archetypeName, previousManufacturerID string, rng *rand.Rand) (manufacturerID string, err error)

var manufacturerPicker ManufacturerPicker

// SetManufacturerPicker is called once at server startup (from the
// factory root package) to wire up manufacturer resolution.
func SetManufacturerPicker(p ManufacturerPicker) { manufacturerPicker = p }

// CivTechTierLookup returns the TechTier (1..5) for a given civ ID.
// Injected for the same import-cycle reason as ManufacturerPicker.
type CivTechTierLookup func(civilizationID string) (techTier int, ok bool)

var civTechTierLookup CivTechTierLookup

// SetCivTechTierLookup wires the civ-tier lookup at server startup. If
// unset, GenerateForSlot treats every archetype as ungated (tier 0) and
// returns an error so the misconfiguration is loud, not silent.
func SetCivTechTierLookup(fn CivTechTierLookup) { civTechTierLookup = fn }

// GenerateForSlot picks an archetype registered for the slot (filtered
// by the civ's TechTier against each archetype's minTechTier), resolves
// a manufacturer for the given civilization, and runs the archetype's
// generator. previousManufacturerID is passed through to the picker as
// a provenance hint — see ManufacturerPicker. The picked manufacturer ID
// is returned so the caller can thread it as the hint to the next slot.
func GenerateForSlot(slot FlightSlot, civilizationID, previousManufacturerID string, civ *CivBias, rng *rand.Rand) (FlightSystem, string, error) {
	_ = civ // commits 5-7 read civ; commit 3 just plumbs it through.
	entries := slotRegistry[slot]
	if len(entries) == 0 {
		return nil, "", ErrSlotEmpty
	}

	// Filter by TechTier. Tier lookup is mandatory — missing wiring is
	// a configuration error, not an excuse to silently ignore gates.
	if civTechTierLookup == nil {
		return nil, "", errors.New("flight: civ tech-tier lookup not configured")
	}
	civTier, ok := civTechTierLookup(civilizationID)
	if !ok {
		return nil, "", fmt.Errorf("flight: unknown civilization %q", civilizationID)
	}
	eligible := make([]archetypeEntry, 0, len(entries))
	for _, e := range entries {
		if e.minTechTier <= civTier {
			eligible = append(eligible, e)
		}
	}
	if len(eligible) == 0 {
		return nil, "", ErrSlotEmpty
	}

	// Weighted archetype pick. rarity 0 → treat as 1.0.
	total := 0.0
	for _, e := range eligible {
		w := e.rarity
		if w <= 0 {
			w = 1.0
		}
		total += w
	}
	r := rng.Float64() * total
	acc := 0.0
	arch := eligible[len(eligible)-1]
	for _, e := range eligible {
		w := e.rarity
		if w <= 0 {
			w = 1.0
		}
		acc += w
		if r < acc {
			arch = e
			break
		}
	}

	if manufacturerPicker == nil {
		return nil, "", errors.New("flight: manufacturer picker not configured")
	}
	mfgID, err := manufacturerPicker(civilizationID, arch.archetypeName, previousManufacturerID, rng)
	if err != nil {
		return nil, "", fmt.Errorf("pick manufacturer: %w", err)
	}
	sys, err := arch.generate(mfgID, rng)
	if err != nil {
		return nil, "", err
	}
	return sys, mfgID, nil
}
