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
	minTechTier   int // 0 = no gate; civ must satisfy civTier >= minTechTier
}

// slotRegistry maps each flight slot to its registered archetypes.
// Populated via register() from per-category init() funcs (e.g. liquid.go).
var slotRegistry = map[FlightSlot][]archetypeEntry{}

// register adds an archetype with no tech-tier gate. Used by liquid
// archetypes, which are available to all civs regardless of tier.
func register(slot FlightSlot, name string, gen archetypeGenerator) {
	registerWithTier(slot, name, gen, 0)
}

// registerWithTier adds a tier-gated archetype. minTechTier 0 means no
// gate; otherwise civs with TechTier < minTechTier never see this
// archetype and their slot rolls as ErrSlotEmpty if nothing else
// survives filtering.
func registerWithTier(slot FlightSlot, name string, gen archetypeGenerator, minTechTier int) {
	slotRegistry[slot] = append(slotRegistry[slot], archetypeEntry{
		archetypeName: name,
		generate:      gen,
		minTechTier:   minTechTier,
	})
}

// ManufacturerPicker resolves a manufacturer for a given civ + archetype.
// Injected so the flight package doesn't import the factory root, which
// would close an import cycle (factory imports flight in commit 8).
type ManufacturerPicker func(civilizationID, archetypeName string, rng *rand.Rand) (manufacturerID string, err error)

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
// generator.
func GenerateForSlot(slot FlightSlot, civilizationID string, rng *rand.Rand) (FlightSystem, error) {
	entries := slotRegistry[slot]
	if len(entries) == 0 {
		return nil, ErrSlotEmpty
	}

	// Filter by TechTier. Tier lookup is mandatory — missing wiring is
	// a configuration error, not an excuse to silently ignore gates.
	if civTechTierLookup == nil {
		return nil, errors.New("flight: civ tech-tier lookup not configured")
	}
	civTier, ok := civTechTierLookup(civilizationID)
	if !ok {
		return nil, fmt.Errorf("flight: unknown civilization %q", civilizationID)
	}
	eligible := make([]archetypeEntry, 0, len(entries))
	for _, e := range entries {
		if e.minTechTier <= civTier {
			eligible = append(eligible, e)
		}
	}
	if len(eligible) == 0 {
		return nil, ErrSlotEmpty
	}

	// Uniform archetype pick. Rarity weights land in commit 8.
	arch := eligible[rng.Intn(len(eligible))]

	if manufacturerPicker == nil {
		return nil, errors.New("flight: manufacturer picker not configured")
	}
	mfgID, err := manufacturerPicker(civilizationID, arch.archetypeName, rng)
	if err != nil {
		return nil, fmt.Errorf("pick manufacturer: %w", err)
	}
	return arch.generate(mfgID, rng)
}
