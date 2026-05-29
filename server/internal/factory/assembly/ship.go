// Package assembly owns ship-level generation — iterating the flight
// slots and producing a ShipLoadout. Lives in its own subpackage
// because it depends on both the factory root and the flight category
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
const FactoryVersion = "phase5_1-v1"

// ShipLoadout is the serialisable ship configuration.
type ShipLoadout struct {
	FactoryVersion string
	Flight         map[flight.FlightSlot]any
}

// MarshalJSON emits:
//
//	{ "factory_version": "...", "flight": { "short": {...}, "medium": ..., "far": ... } }
//
// All three slots are always present in the JSON; an empty slot serialises
// as explicit null rather than being dropped from the map.
func (l ShipLoadout) MarshalJSON() ([]byte, error) {
	flightMap := map[string]any{}
	for _, slot := range []flight.FlightSlot{flight.Short, flight.Medium, flight.Far} {
		flightMap[slot.String()] = l.Flight[slot]
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
// selection consult its TechProfile and every part is stamped with the
// civ's name as manufacturer. When nil, the roll falls back to a
// tier-3 baseline with no bias. Deterministic: same (seed, civ) →
// same ship.
func GenerateRandomShip(seed int64, civ *factory.Civilization) (*ShipLoadout, error) {
	rng := rand.New(rand.NewSource(seed))
	bias := civBiasFor(civ)

	loadout := &ShipLoadout{
		FactoryVersion: FactoryVersion,
		Flight:         map[flight.FlightSlot]any{},
	}

	for _, slot := range []flight.FlightSlot{flight.Short, flight.Medium, flight.Far} {
		sys, err := flight.GenerateForSlot(slot, bias, rng)
		switch {
		case err == nil:
			loadout.Flight[slot] = sys
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
// dispatcher reads, plus the manufacturer-stamping fields (civ name and
// derived prefix). Returns nil for nil civ — the dispatcher treats
// nil as "no bias, tier 3, generic prefix."
func civBiasFor(civ *factory.Civilization) *flight.CivBias {
	if civ == nil {
		return nil
	}
	tp := civ.TechProfile
	bias := &flight.CivBias{
		Name:                  civ.Name,
		ManufacturerPrefix:    factory.ShipwrightPrefix(civ.Name),
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
