// Package assembly owns ship-level generation — picking a primary
// civilization, iterating the flight slots, and producing a persisted
// ShipLoadout. Lives in its own subpackage because it depends on both
// the factory root (registries, GenContext) and the flight category
// subpackage. Putting it at the factory root would close an import
// cycle with flight/.
package assembly

import (
	"encoding/json"
	"errors"
	"math/rand"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// FactoryVersion tags every generated loadout so we can reason about
// ships produced before a later factory change. Bump the suffix whenever
// generation behavior changes in a way that invalidates old loadouts.
const FactoryVersion = "phase5-v1"

// ShipLoadout is the serialisable, JSONB-persisted ship configuration.
// Phase 3 only populates the Short flight slot; Medium and Far are
// always nil — they serialise as explicit JSON null so the frontend
// contract is stable (keys always present).
type ShipLoadout struct {
	FactoryVersion string
	// TODO(phase-4): tighten to map[flight.FlightSlot]flight.FlightSystem
	// once FlightSystem grows methods.
	Flight map[flight.FlightSlot]any
}

// MarshalJSON emits the shape documented in Plan §4:
//
//	{
//	  "factory_version": "...",
//	  "flight": { "short": {...}, "medium": null, "far": null }
//	}
func (l ShipLoadout) MarshalJSON() ([]byte, error) {
	flightMap := map[string]any{}
	for _, slot := range []flight.FlightSlot{flight.Short, flight.Medium, flight.Far} {
		flightMap[slot.String()] = l.Flight[slot] // nil → JSON null
	}
	return json.Marshal(struct {
		FactoryVersion string         `json:"factory_version"`
		Flight         map[string]any `json:"flight"`
	}{
		FactoryVersion: l.FactoryVersion,
		Flight:         flightMap,
	})
}

// GenerateRandomShip is the single top-level entry point called by the
// /api/ships/generate handler. When civ is non-nil, archetype + mixture
// selection consult its TechProfile (commits 5–7); when nil, the roll
// falls back to GenericCivilization with no bias. Deterministic: same
// (seed, civ) → same ship.
func GenerateRandomShip(seed int64, civ *factory.Civilization) (*ShipLoadout, error) {
	rng := rand.New(rand.NewSource(seed))
	primaryCivID := factory.GenericCivilizationID
	if civ != nil {
		primaryCivID = civ.ID
	}
	bias := civBiasFor(civ)

	loadout := &ShipLoadout{
		FactoryVersion: FactoryVersion,
		Flight:         map[flight.FlightSlot]any{},
	}

	// previousMfg threads the manufacturer picked for the previous slot
	// into the next slot's picker as a provenance hint. Empty on the
	// first slot and preserved across ErrSlotEmpty slots so a ship with
	// Short+Far (Medium empty) still biases Far toward Short's vendor.
	previousMfg := ""
	for _, slot := range []flight.FlightSlot{flight.Short, flight.Medium, flight.Far} {
		sys, mfgID, err := flight.GenerateForSlot(slot, primaryCivID, previousMfg, bias, rng)
		switch {
		case err == nil:
			loadout.Flight[slot] = sys
			previousMfg = mfgID
		case errors.Is(err, flight.ErrSlotEmpty):
			// Explicit nil so MarshalJSON emits "slot": null rather
			// than dropping the key.
			loadout.Flight[slot] = nil
		default:
			return nil, err
		}
	}

	return loadout, nil
}

// civBiasFor projects the subset of a civ's TechProfile the flight
// dispatcher reads. Returns nil for nil civ — the dispatcher treats
// nil as "no bias."
func civBiasFor(civ *factory.Civilization) *flight.CivBias {
	if civ == nil {
		return nil
	}
	tp := civ.TechProfile
	bias := &flight.CivBias{
		TechTier:              civ.TechTier,
		RiskTolerance:         tp.RiskTolerance,
		ThrustVsIspPreference: tp.ThrustVsIspPreference,
		AversionToCryogenics:  tp.AversionToCryogenics,
	}
	if len(tp.PreferredMixtureIDs) > 0 {
		bias.PreferredMixtureIDs = make(map[string]bool, len(tp.PreferredMixtureIDs))
		for _, id := range tp.PreferredMixtureIDs {
			bias.PreferredMixtureIDs[id] = true
		}
	}
	if len(tp.PreferredCoolingMethods) > 0 {
		bias.PreferredCoolingMethods = make(map[string]bool, len(tp.PreferredCoolingMethods))
		for _, c := range tp.PreferredCoolingMethods {
			bias.PreferredCoolingMethods[c.String()] = true
		}
	}
	if len(tp.PreferredIgnitionTypes) > 0 {
		bias.PreferredIgnitionTypes = make(map[string]bool, len(tp.PreferredIgnitionTypes))
		for _, i := range tp.PreferredIgnitionTypes {
			bias.PreferredIgnitionTypes[i.String()] = true
		}
	}
	return bias
}
