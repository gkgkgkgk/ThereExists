package factory

type Mixture struct {
	ID              string
	Config          PropellantConfig
	IspMultiplier   float64
	DensityKgM3     float64
	StorabilityDays int  // -1 = indefinite
	Hypergolic      bool // ignites on contact → forces IgnitionMethod = Hypergolic
	Cryogenic       bool // requires active cooling; typically caps restarts
}

// Mixtures is the hand-authored propellant registry. Kept small and
// hand-authored because mixtures propagate cross-category: an engine's
// MixtureID must match a tank the ship will later carry (Plan §2).
var Mixtures = map[string]*Mixture{}

func init() {
	reg := func(m *Mixture) { Mixtures[m.ID] = m }

	reg(&Mixture{
		ID:              "LOX_LH2",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     360,
		StorabilityDays: -1,
		Hypergolic:      false,
		Cryogenic:       true,
	})

	reg(&Mixture{
		ID:              "LOX_RP1",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1030,
		StorabilityDays: -1,
		Hypergolic:      false,
		Cryogenic:       true,
	})

	reg(&Mixture{
		ID:              "MMH_NTO",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1190,
		StorabilityDays: 3650, // ~10 years
		Hypergolic:      true,
		Cryogenic:       false,
	})

	reg(&Mixture{
		ID:              "Hydrazine",
		Config:          Monopropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     1021,
		StorabilityDays: 3650,
		Hypergolic:      false,
		Cryogenic:       false,
	})
}
