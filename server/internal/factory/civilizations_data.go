package factory

// Phase 3 ships a single hand-authored civilization so the provenance
// chain (Manufacturer → Civilization → TechTier) is exercised end-to-end.
// Phase 4 replaces this file with LLM-generated civilizations.

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
