package flight

// CivBias is the read-only subset of a civilization's TechProfile that
// the flight dispatcher consults during archetype + mixture selection.
// Defined here (not in factory) to keep the flight package leaf-y — the
// factory root constructs CivBias values and passes them through, the
// same indirection ManufacturerPicker and CivTechTierLookup use to
// avoid closing an import cycle.
//
// A nil *CivBias means "no bias" — every selection collapses to the
// pre-civ behavior. Callers (assembly) build a fresh CivBias per ship
// roll; CivBias is treated as immutable after construction.
type CivBias struct {
	TechTier              int
	RiskTolerance         float64
	ThrustVsIspPreference float64
	AversionToCryogenics  float64

	// Set semantics — the picker does O(1) `if x[id]` lookups. Empty /
	// nil maps mean "no preference," not "exclude everything."
	PreferredMixtureIDs     map[string]bool
	PreferredCoolingMethods map[string]bool // CoolingMethod.String() keys
	PreferredIgnitionTypes  map[string]bool // IgnitionMethod.String() keys
}
