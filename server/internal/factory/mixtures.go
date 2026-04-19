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

// Mixtures is the hand-authored propellant registry. Populated in
// mixtures_data.go (commit 4).
var Mixtures = map[string]*Mixture{}
