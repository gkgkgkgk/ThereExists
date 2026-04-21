// Package refinery owns the ship-borne refinery subsystem that turns
// wild precursors into finished propellants. Phase 4 lands only the
// type system and an empty archetype registry — no instances are
// authored and ShipLoadout does NOT yet carry a Refinery field.
// Content (archetypes + their Productions) is filled in a post-infra
// pass; see Phase 4 Plan §3.
//
// Multi-level model: a mixture is produced by zero or more refinery
// archetypes, each with its own Recipe / Catalyst / Power / Throughput
// profile. Recipes therefore live on MixtureProduction (refinery-side),
// NOT on factory.Mixture.
package refinery

import (
	"errors"
	"fmt"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// ──────────────────────────── Archetype ────────────────────────────────

// MixtureProduction describes how one RefineryArchetype produces one
// Mixture. The same MixtureID can appear on multiple archetypes with
// different Recipe / Catalyst / Power / Throughput — that's the whole
// point of the multi-level refinery design.
type MixtureProduction struct {
	MixtureID string

	// Recipe names the WILD PRECURSOR inputs needed per kg of finished
	// propellant. Must bottom out at WildPrecursor resources — refined
	// chemicals are not resources and cannot appear here.
	Recipe []factory.ResourceInput

	// CatalystID is the catalyst this specific production consumes.
	// Different productions of the same mixture can use different
	// catalysts (and different wear rates).
	CatalystID       factory.ResourceID
	CatalystUsePerKg float64

	PowerDrawWRange       [2]float64 // watts while this mixture is being produced
	ThroughputKgHourRange [2]float64 // kg finished propellant per hour
}

// RefineryArchetype is a static template for a class of refinery. A
// ship rolls one Refinery from one archetype (future Phase).
type RefineryArchetype struct {
	Name        string
	Description string

	TechTier            int // 1..5; mirrors civ / flight archetype gating
	HealthInitRange     [2]float64
	DryMassKgRange      [2]float64
	IdlePowerDrawWRange [2]float64 // containment / thermal draw when not refining

	// Productions is the full list of (mixture, recipe) pairs this
	// archetype can produce. Non-empty for every registered archetype.
	Productions []MixtureProduction
}

// ──────────────────────────── Instance ─────────────────────────────────

// Refinery is a concrete rolled refinery. Not produced in Phase 4 beyond
// the type existing — no ShipLoadout field yet.
type Refinery struct {
	factory.SystemBase

	Health         []float64
	DryMassKg      float64
	IdlePowerDrawW float64

	// Productions is copied from the archetype so a persisted ship
	// loadout is self-describing (no re-reading factory data at load).
	Productions []MixtureProduction
}

// ──────────────────────────── Validation ───────────────────────────────

// ErrMixtureNotAuthored signals that a production references a mixture
// not yet present in factory.Mixtures. Returned (not panicked) so
// infrastructure can land before content. Analogous to the relaxed
// validator on the flight package.
var ErrMixtureNotAuthored = errors.New("refinery: mixture not authored")

// Validate enforces structural invariants. Safe on an empty archetype
// (no Productions) only when the archetype is NOT being registered —
// the registration path separately enforces Productions non-empty.
// Resource / mixture lookups are empty-registry-safe: a missing
// reference produces an error, not a panic.
func (a RefineryArchetype) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("refinery: archetype has empty Name")
	}
	if a.TechTier < 1 || a.TechTier > 5 {
		return fmt.Errorf("refinery: archetype %q TechTier %d outside [1,5]", a.Name, a.TechTier)
	}
	if err := checkRange("HealthInitRange", a.HealthInitRange); err != nil {
		return fmt.Errorf("refinery: archetype %q: %w", a.Name, err)
	}
	if err := checkRange("DryMassKgRange", a.DryMassKgRange); err != nil {
		return fmt.Errorf("refinery: archetype %q: %w", a.Name, err)
	}
	if err := checkRange("IdlePowerDrawWRange", a.IdlePowerDrawWRange); err != nil {
		return fmt.Errorf("refinery: archetype %q: %w", a.Name, err)
	}

	for i, p := range a.Productions {
		if p.MixtureID == "" {
			return fmt.Errorf("refinery: archetype %q production[%d] has empty MixtureID", a.Name, i)
		}
		if _, ok := factory.LookupMixture(p.MixtureID); !ok {
			return fmt.Errorf("refinery: archetype %q production[%d] mixture %q: %w", a.Name, i, p.MixtureID, ErrMixtureNotAuthored)
		}
		if err := checkRange("PowerDrawWRange", p.PowerDrawWRange); err != nil {
			return fmt.Errorf("refinery: archetype %q production[%d] (%s): %w", a.Name, i, p.MixtureID, err)
		}
		if err := checkRange("ThroughputKgHourRange", p.ThroughputKgHourRange); err != nil {
			return fmt.Errorf("refinery: archetype %q production[%d] (%s): %w", a.Name, i, p.MixtureID, err)
		}
		if p.CatalystUsePerKg < 0 {
			return fmt.Errorf("refinery: archetype %q production[%d] (%s) CatalystUsePerKg is negative", a.Name, i, p.MixtureID)
		}
		// Recipe entries must resolve to WildPrecursor resources. Empty
		// recipe is allowed at validation time (edge case) — content
		// pass will tighten this.
		for j, ri := range p.Recipe {
			if ri.ResourceID == "" {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) recipe[%d] has empty ResourceID", a.Name, i, p.MixtureID, j)
			}
			r, ok := factory.LookupResource(ri.ResourceID)
			if !ok {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) recipe[%d] unknown resource %q", a.Name, i, p.MixtureID, j, ri.ResourceID)
			}
			if r.Category != factory.WildPrecursor {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) recipe[%d] resource %q has category %s (must be WildPrecursor)", a.Name, i, p.MixtureID, j, ri.ResourceID, r.Category)
			}
			if ri.QuantityPerUnitFuel <= 0 {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) recipe[%d] QuantityPerUnitFuel %v must be > 0", a.Name, i, p.MixtureID, j, ri.QuantityPerUnitFuel)
			}
		}
		if p.CatalystID != "" {
			r, ok := factory.LookupResource(p.CatalystID)
			if !ok {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) unknown CatalystID %q", a.Name, i, p.MixtureID, p.CatalystID)
			}
			if r.Category != factory.Catalyst {
				return fmt.Errorf("refinery: archetype %q production[%d] (%s) CatalystID %q has category %s (must be Catalyst)", a.Name, i, p.MixtureID, p.CatalystID, r.Category)
			}
		}
	}
	return nil
}

func checkRange(name string, r [2]float64) error {
	if r[1] < r[0] {
		return fmt.Errorf("%s: hi (%v) < lo (%v)", name, r[1], r[0])
	}
	return nil
}

// ──────────────────────────── Registry ─────────────────────────────────

var registeredArchetypes []RefineryArchetype

// registerRefineryArchetype appends an archetype to the registry.
// Enforces non-empty Productions at registration time (the Validate()
// method is permissive on empty Productions so the empty-registry path
// works cleanly during package init).
func registerRefineryArchetype(a RefineryArchetype) {
	if len(a.Productions) == 0 {
		panic(fmt.Sprintf("refinery: archetype %q has no Productions", a.Name))
	}
	for _, existing := range registeredArchetypes {
		if existing.Name == a.Name {
			panic(fmt.Sprintf("refinery: duplicate archetype name %q", a.Name))
		}
	}
	registeredArchetypes = append(registeredArchetypes, a)
}

// Archetypes returns a read-only view of all registered refinery
// archetypes. Empty in Phase 4.
func Archetypes() []RefineryArchetype {
	return registeredArchetypes
}

// SupportedMixtureIDs returns the unique mixture IDs this archetype can
// produce. Derived from Productions.
func (a RefineryArchetype) SupportedMixtureIDs() []string {
	seen := make(map[string]struct{}, len(a.Productions))
	out := make([]string, 0, len(a.Productions))
	for _, p := range a.Productions {
		if _, dup := seen[p.MixtureID]; dup {
			continue
		}
		seen[p.MixtureID] = struct{}{}
		out = append(out, p.MixtureID)
	}
	return out
}

func init() {
	// Refinery archetype registry starts empty — content is authored in
	// a post-infra pass. The loop below is a no-op today but wired so
	// future additions Just Work: a bad archetype trips package init,
	// not a random request.
	for _, a := range registeredArchetypes {
		if err := a.Validate(); err != nil {
			panic(fmt.Sprintf("refinery: archetype %q failed validation: %v", a.Name, err))
		}
	}
}
