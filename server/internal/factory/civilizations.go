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

// Civilization describes a generated society. Phase 5 adds Name,
// Description, HomeworldDescription, HomeworldPlanetID, and AgeYears;
// civ generation is now an LLM pipeline (see civgen.go).
type Civilization struct {
	ID                   string
	Name                 string
	Description          string
	HomeworldDescription string
	// HomeworldPlanetID is reserved for the planet-generation phase. Nil
	// in Phase 5 because no planet registry exists yet. Non-nil means
	// "this civ references a registered homeworld."
	HomeworldPlanetID *string
	AgeYears          int64
	TechTier          int
	TechProfile       TechProfile
	// Flavor is a short one-liner used in UI chips / summaries.
	// Distinct from Description, which is the long-form prose.
	Flavor string
}

// Civilizations is the hand-authored registry. Phase 5 keeps
// GenericCivilization as a fallback and test fixture even though civs
// are now primarily LLM-generated.
var Civilizations = map[string]*Civilization{}

const GenericCivilizationID = "GenericCivilization"

func init() {
	Civilizations[GenericCivilizationID] = &Civilization{
		ID:   GenericCivilizationID,
		Name: "Generic Civilization",
		Description: "A broadly competent spacefaring culture with no strong biases toward any single propulsion family or thermal strategy. " +
			"They build reliable, mid-tech ships assembled from parts sourced across their trade network — rarely elegant, rarely brittle. " +
			"The civilization is old enough to have accumulated engineering discipline but young enough that its institutions still innovate.",
		HomeworldDescription: "Temperate terrestrial homeworld with an Earth-analogue atmosphere, liquid water, and a stable magnetosphere.",
		HomeworldPlanetID:    nil,
		AgeYears:             5000,
		TechTier:             3, // middle of a 1–5 scale
		TechProfile: TechProfile{
			// Empty preference lists = no bias — every option is equally
			// likely for this civ. LLM-generated civs narrow these.
			PreferredCoolingMethods: nil,
			PreferredIgnitionTypes:  nil,
			PreferredMixtureIDs:     nil,
			AversionToCryogenics:    0.0,
			FarDriveFamily:          "",
		},
		Flavor: "Default mid-tier civ for development and fallback.",
	}
}
