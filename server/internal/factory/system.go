package factory

import "github.com/google/uuid"

// SystemBase is embedded by every concrete system instance. Quality
// inherits from the civ — there's no separate manufacturer roster
// anymore, so ManufacturerName is just the civ's display name stamped
// onto the part for UI / flavor.
//
// Redundancy model: a system represents a group of Count identical
// physical units (e.g. four identical RCS thrusters). Controls treat the
// group as one — summed/averaged over healthy units — but damage and
// repair act per-unit via the Health slice (len == Count, each in [0,1]).
//
// Future: the plain per-unit float will grow into a richer UnitState
// holding a break *reason* (fuel_line, ignition, pump, ...) so different
// breaks can drive different repair flows. Migrate with a FactoryVersion
// bump when the damage/repair sim lands.
type SystemBase struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	ArchetypeName    string    `json:"archetype"`
	ManufacturerName string    `json:"manufacturer"`
	SerialNumber     string    `json:"serial_number"`
	Count            int       `json:"count"`
	Health           []float64 `json:"health"`
}
