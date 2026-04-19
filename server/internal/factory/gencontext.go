package factory

import "math/rand"

// GenContext threads the two things a category generator needs — the
// chosen manufacturer and the caller-owned RNG — through the dispatcher.
// Lives in the factory root so category subpackages can embed it
// without importing each other.
type GenContext struct {
	ManufacturerID string
	Rng            *rand.Rand
}
