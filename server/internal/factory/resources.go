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

// Resources is the hand-authored resource registry. Phase 4 ships it
// empty — content is authored in a post-infra pass. Validation below
// is empty-registry-safe.
var Resources = map[ResourceID]*Resource{}

func registerResource(r *Resource) {
	if r == nil {
		panic("factory: registerResource received nil")
	}
	if r.ID == "" {
		panic("factory: resource has empty ID")
	}
	if _, dup := Resources[r.ID]; dup {
		panic(fmt.Sprintf("factory: duplicate resource ID %q", r.ID))
	}
	if r.DisplayName == "" {
		panic(fmt.Sprintf("factory: resource %q has empty DisplayName", r.ID))
	}
	if r.Commonality < 1 || r.Commonality > 5 {
		panic(fmt.Sprintf("factory: resource %q has Commonality %d outside [1,5]", r.ID, r.Commonality))
	}
	Resources[r.ID] = r
}

// LookupResource returns the resource for the given ID. Safe on an empty
// registry — callers get (nil, false) rather than a panic.
func LookupResource(id ResourceID) (*Resource, bool) {
	r, ok := Resources[id]
	return r, ok
}

func init() {
	// Resource registry starts empty — content is authored in a post-infra pass.
	// Validation below runs over whatever is registered; empty registry passes trivially.
	for _, r := range Resources {
		if r.DisplayName == "" {
			panic(fmt.Sprintf("factory: resource %q has empty DisplayName", r.ID))
		}
		if r.Commonality < 1 || r.Commonality > 5 {
			panic(fmt.Sprintf("factory: resource %q has Commonality %d outside [1,5]", r.ID, r.Commonality))
		}
	}
}
