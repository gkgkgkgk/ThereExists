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
	// Name is the civ's display name. Stamped onto every generated
	// system as the manufacturer (one civ = one shipwright in this
	// universe; no separate manufacturer roster).
	Name string
	// ManufacturerPrefix is the 2–4 letter code derived from Name —
	// precomputed by the assembly layer (factory.ShipwrightPrefix) so
	// flight generators don't need to call back into factory for each
	// part's serial number.
	ManufacturerPrefix string

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

// manufacturerStamp returns the (manufacturerName, prefix, techTier)
// triple a generator stamps onto a SystemBase. Centralised so the
// nil-civ fallback ("generic shipwright at tier 3") doesn't need to
// be reimplemented in each per-category generator.
func manufacturerStamp(civ *CivBias) (name, prefix string, tier int) {
	if civ == nil {
		return "Generic Shipwright", "GEN", defaultTechTier
	}
	prefix = civ.ManufacturerPrefix
	if prefix == "" {
		prefix = "GEN"
	}
	name = civ.Name
	if name == "" {
		name = "Unknown"
	}
	return name, prefix, civ.TechTier
}
