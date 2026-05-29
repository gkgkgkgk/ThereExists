package factory

// TechProfile captures a civilization's cultural/scientific preferences —
// orthogonal to TechTier (which ladders engineering advancement).
type TechProfile struct {
	// DesignPhilosophy is a short phrase capturing the civ's engineering
	// ethos — e.g. "utilitarian austerity", "baroque redundancy",
	// "biomorphic grown-hulls", "ritualised conservatism". Generated
	// alongside the description (step 2) and fed into step 4 so the
	// constrained-choice LLM can reason about *why* this civ picks the
	// cooling method / mixture / ignition it does, not just from planet
	// conditions alone.
	DesignPhilosophy        string
	PreferredCoolingMethods []CoolingMethod
	PreferredIgnitionTypes  []IgnitionMethod
	PreferredMixtureIDs     []string
	AversionToCryogenics    float64
	FarDriveFamily          string

	// RiskTolerance is in [0, 1]. 0 = ultra-conservative (only proven
	// archetypes, fully-healthy starting parts, no exotic chemistry).
	// 1 = experimental (accepts fragile/rare archetypes, lower starting
	// health, novel mixtures). Used by ship generation to weight
	// archetype rarity and seed the HealthInitRange roll.
	RiskTolerance float64

	// ThrustVsIspPreference is in [-1, 1]. -1 = punchy short bursts
	// (high T/W, low Isp — RDE, RCS-heavy). +1 = efficient long burns
	// (high Isp, lower T/W — SCTA, ion-adjacent). 0 = balanced.
	// Biases archetype selection among equally-valid candidates in a
	// slot.
	ThrustVsIspPreference float64
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

