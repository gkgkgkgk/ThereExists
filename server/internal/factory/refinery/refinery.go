// Package refinery owns the ship-borne refinery subsystem. Phase 4.1
// demotes the refinery from recipe owner to *modulator*: the recipe
// (precursors, power, time, catalyst identity) lives on factory.Mixture;
// the refinery contributes efficiency, throughput cap, heat output, and
// catalyst wear state. Gating still lives here via SupportedMixtureIDs —
// an archetype declares which chemistries it can run at all.
//
// Registry starts empty; content (archetypes) is authored in a
// post-infra pass. ShipLoadout does NOT yet carry a Refinery field.
package refinery

import (
	"fmt"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// ──────────────────────────── Archetype ────────────────────────────────

// RefineryArchetype is a static template for a class of refinery. A
// ship rolls one Refinery from one archetype (future Phase).
//
// The archetype does NOT own recipes. It declares which mixtures it
// supports (gating) and the ranges from which its modulation scalars
// are rolled at ship-generation time. The mixture owns the chemistry.
type RefineryArchetype struct {
	Name        string
	Description string

	TechTier            int // 1..5; mirrors civ / flight archetype gating
	HealthInitRange     [2]float64
	DryMassKgRange      [2]float64
	IdlePowerDrawWRange [2]float64 // containment / thermal draw when not refining

	// Modulation ranges — rolled once per Refinery instance.
	EfficiencyRange      [2]float64 // 0..1 yield multiplier applied to Mixture.Precursors
	ThroughputLimitRange [2]float64 // hard kg/hr cap, independent of mixture
	HeatOutputPerWRange  [2]float64 // thermal cost per watt drawn while refining

	// Gating — mixture IDs this archetype is allowed to run. A mixture
	// not in this list is refused even if the ship has the precursors.
	// The mixture owns the recipe; this archetype owns whether it's
	// *willing* to cook it.
	SupportedMixtureIDs []string
}

// ──────────────────────────── Instance ─────────────────────────────────

// Refinery is a concrete rolled refinery. Not produced in Phase 4.1
// beyond the type existing — no ShipLoadout field yet.
type Refinery struct {
	factory.SystemBase

	Health         []float64
	DryMassKg      float64
	IdlePowerDrawW float64

	// Modulation scalars — rolled once from the archetype's ranges.
	Efficiency      float64 // 0..1 yield multiplier on mixture precursors
	ThroughputLimit float64 // kg/hr cap
	HeatOutputPerW  float64

	// CatalystHealth 0..1; wears with use (runtime, later phase).
	// The *identity* of the catalyst comes from the current Mixture;
	// the wear state lives here because it's a property of *this
	// refinery's* hardware, not of the chemistry.
	CatalystHealth float64

	// SupportedMixtureIDs is copied from the archetype so a persisted
	// ship loadout is self-describing (no re-reading factory data on load).
	SupportedMixtureIDs []string
}

// ──────────────────────────── Validation ───────────────────────────────

// Validate enforces structural invariants carried forward from Phase 4.
// Modulation-range and cross-package reachability checks land in the
// next commit (Phase 4.1 §4); this commit is schema-only.
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
// Enforces non-empty SupportedMixtureIDs at registration time (an
// archetype that supports no mixtures is useless); Validate() itself
// stays permissive so the empty-registry init path works cleanly.
func registerRefineryArchetype(a RefineryArchetype) {
	if len(a.SupportedMixtureIDs) == 0 {
		panic(fmt.Sprintf("refinery: archetype %q has no SupportedMixtureIDs", a.Name))
	}
	for _, existing := range registeredArchetypes {
		if existing.Name == a.Name {
			panic(fmt.Sprintf("refinery: duplicate archetype name %q", a.Name))
		}
	}
	registeredArchetypes = append(registeredArchetypes, a)
}

// Archetypes returns a read-only view of all registered refinery
// archetypes. Empty in Phase 4.1.
func Archetypes() []RefineryArchetype {
	return registeredArchetypes
}

func init() {
	// Registry starts empty; loop is a no-op today but wired so future
	// additions trip init on bad data rather than a random request.
	for _, a := range registeredArchetypes {
		if err := a.Validate(); err != nil {
			panic(fmt.Sprintf("refinery: archetype %q failed validation: %v", a.Name, err))
		}
	}
}
