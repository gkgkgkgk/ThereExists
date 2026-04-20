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

// Civilizations is the hand-authored registry. Phase 3 ships a single
// entry (GenericCivilization); Phase 4 replaces this with LLM-generated
// civilizations.
var Civilizations = map[string]*Civilization{}

const GenericCivilizationID = "GenericCivilization"

func init() {
	Civilizations[GenericCivilizationID] = &Civilization{
		ID:          GenericCivilizationID,
		DisplayName: "Generic Civilization",
		TechTier:    3, // middle of a 1–5 scale
		TechProfile: TechProfile{
			// Empty preference lists = no bias — every option is equally
			// likely for this civ. Phase 4 civs will narrow these.
			PreferredCoolingMethods: nil,
			PreferredIgnitionTypes:  nil,
			PreferredMixtureIDs:     nil,
			AversionToCryogenics:    0.0,
			FarDriveFamily:          "",
		},
		Flavor: "A default mid-tier civilization used during Phase 3 development. " +
			"Phase 4 replaces this with LLM-generated civilizations.",
	}
}
