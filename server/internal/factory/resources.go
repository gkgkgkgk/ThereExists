package factory

import "fmt"

// ResourceID identifies a resource (wild precursor, catalyst, or ignition
// component). Named type — not a bare string alias — so the compiler
// catches misuse (e.g. passing a mixture ID where a resource ID is
// expected). See Phase 4 Plan §2.
type ResourceID string

// ResourceCategory classifies what role a resource plays in the metabolic
// loop. Wild precursors are raw mass found in the void; catalysts wear
// with use (either in a refinery or an engine); ignition components are
// one-shot-ish hardware needed to light a burn. Refined chemicals
// (LOX, LH2, MMH, etc.) are NOT resources — they are mixtures produced
// by refineries from wild precursors.
type ResourceCategory int

const (
	WildPrecursor ResourceCategory = iota
	Catalyst
	IgnitionComponent
)

func (c ResourceCategory) String() string {
	switch c {
	case WildPrecursor:
		return "WildPrecursor"
	case Catalyst:
		return "Catalyst"
	case IgnitionComponent:
		return "IgnitionComponent"
	default:
		return fmt.Sprintf("ResourceCategory(%d)", int(c))
	}
}

type PhaseOfMatter int

const (
	Solid PhaseOfMatter = iota
	Liquid
	Gas
	Plasma
	Exotic
)

func (p PhaseOfMatter) String() string {
	switch p {
	case Solid:
		return "Solid"
	case Liquid:
		return "Liquid"
	case Gas:
		return "Gas"
	case Plasma:
		return "Plasma"
	case Exotic:
		return "Exotic"
	default:
		return fmt.Sprintf("PhaseOfMatter(%d)", int(p))
	}
}

type Resource struct {
	ID                ResourceID
	DisplayName       string
	Category          ResourceCategory
	Phase             PhaseOfMatter
	Commonality       int // 1..5; 1 = ubiquitous, 5 = effectively unobtainable
	TypicalSourceHint string
}

// ResourceInput is a (resource, per-kg quantity) pair. Used by refinery
// productions to declare wild-precursor inputs per kg of finished
// propellant. Lives in the factory package (rather than refinery) so
// non-refinery consumers can reference it without importing refinery.
type ResourceInput struct {
	ResourceID          ResourceID
	QuantityPerUnitFuel float64
}

// Resources is the hand-authored resource registry. Populated via a
// package-level variable initializer (buildResources) so entries are
// present *before* any init() runs — the mixtures.go init calls
// LookupResource during validation, and Go does not guarantee init()
// ordering between files in a package. Variable initialization, by
// contrast, always precedes init().
var Resources = buildResources()

func registerResourceInto(m map[ResourceID]*Resource, r *Resource) {
	if r == nil {
		panic("factory: registerResource received nil")
	}
	if r.ID == "" {
		panic("factory: resource has empty ID")
	}
	if _, dup := m[r.ID]; dup {
		panic(fmt.Sprintf("factory: duplicate resource ID %q", r.ID))
	}
	if r.DisplayName == "" {
		panic(fmt.Sprintf("factory: resource %q has empty DisplayName", r.ID))
	}
	if r.Commonality < 1 || r.Commonality > 5 {
		panic(fmt.Sprintf("factory: resource %q has Commonality %d outside [1,5]", r.ID, r.Commonality))
	}
	m[r.ID] = r
}

// LookupResource returns the resource for the given ID. Safe on an empty
// registry — callers get (nil, false) rather than a panic.
func LookupResource(id ResourceID) (*Resource, bool) {
	r, ok := Resources[id]
	return r, ok
}

// buildResources populates the registry at package-variable-init time
// (i.e. before any init() runs), so mixtures.go can LookupResource
// during its own validation pass without depending on init() ordering.
func buildResources() map[ResourceID]*Resource {
	m := map[ResourceID]*Resource{}
	reg := func(r *Resource) { registerResourceInto(m, r) }

	// Wild precursors — volatiles harvested from comets, asteroids, and
	// icy moons. QuantityPerUnitFuel in Mixture.Precursors is in the
	// same units (kg-ish per kg of finished propellant).
	reg(&Resource{
		ID:                "H2O_ICE",
		DisplayName:       "Water Ice",
		Category:          WildPrecursor,
		Phase:             Solid,
		Commonality:       1,
		TypicalSourceHint: "Cometary ice, outer-system moons, shadowed craters.",
	})
	reg(&Resource{
		ID:                "CH4_ICE",
		DisplayName:       "Methane Ice",
		Category:          WildPrecursor,
		Phase:             Solid,
		Commonality:       2,
		TypicalSourceHint: "Outer-system bodies; Titan-class moons; cold comet cores.",
	})
	reg(&Resource{
		ID:                "NH3_ICE",
		DisplayName:       "Ammonia Ice",
		Category:          WildPrecursor,
		Phase:             Solid,
		Commonality:       3,
		TypicalSourceHint: "Cryovolcanic moons; outer-belt cometary inclusions.",
	})
	reg(&Resource{
		ID:                "N2_ICE",
		DisplayName:       "Nitrogen Ice",
		Category:          WildPrecursor,
		Phase:             Solid,
		Commonality:       3,
		TypicalSourceHint: "Cold outer-system bodies; Pluto-class surfaces.",
	})

	// Ignition hardware. SPARK is a consumable chemical starter;
	// SILVER is a catalyst bed (wears rather than is consumed, but
	// category-wise it's still a Catalyst — IgnitionConfig treats both
	// the same and QuantityPerStart carries the wear-vs-consume amount).
	reg(&Resource{
		ID:                "SPARK",
		DisplayName:       "Chemical Starter Cartridge",
		Category:          IgnitionComponent,
		Phase:             Solid,
		Commonality:       2,
		TypicalSourceHint: "Standard-issue igniter stock; manufactured, not harvested.",
	})
	reg(&Resource{
		ID:                "SILVER",
		DisplayName:       "Silver Catalyst Bed",
		Category:          Catalyst,
		Phase:             Solid,
		Commonality:       3,
		TypicalSourceHint: "Refined silver; salvaged from asteroid-belt processing or pre-authored hardware stock.",
	})

	return m
}
