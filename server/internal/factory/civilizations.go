package factory

// TechProfile captures a civilization's cultural/scientific preferences —
// orthogonal to TechTier (which ladders engineering advancement).
type TechProfile struct {
	PreferredCoolingMethods []CoolingMethod
	PreferredIgnitionTypes  []IgnitionMethod
	PreferredMixtureIDs     []string
	AversionToCryogenics    float64
	FarDriveFamily          string
}

type Civilization struct {
	ID          string
	DisplayName string
	TechTier    int
	TechProfile TechProfile
	Flavor      string
}

// Civilizations is the hand-authored registry. Phase 3 ships exactly one
// entry (GenericCivilization); Phase 4 replaces this with LLM-generated
// civs. See civilizations_data.go.
var Civilizations = map[string]*Civilization{}
