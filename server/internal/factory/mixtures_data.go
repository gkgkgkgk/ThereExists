package factory

// Phase 3 propellant mixture registry. Kept small and hand-authored
// because mixtures propagate cross-category: an engine's MixtureID must
// match a tank the ship will later carry (Plan §2 Mixtures).

func init() {
	reg := func(m *Mixture) { Mixtures[m.ID] = m }

	reg(&Mixture{
		ID:              "LOX_LH2",
		Config:          Bipropellant,
		IspMultiplier:   1.0,
		DensityKgM3:     360, // bulk density of mixed flow
		StorabilityDays: -1,  // indefinite given active cooling; runtime caps restarts
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
		StorabilityDays: 3650, // storable for ~10 years
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
