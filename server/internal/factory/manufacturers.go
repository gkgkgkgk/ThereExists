package factory

import "math/rand"

type Manufacturer struct {
	ID               string
	CivilizationID   string
	DisplayName      string
	NamingConvention func(rng *rand.Rand, archetype string) string
	ArchetypeWeights map[string]float64
	Flavor           string
}

// Manufacturers is the hand-authored registry. Populated in
// manufacturers_data.go (commit 4).
var Manufacturers = map[string]*Manufacturer{}
