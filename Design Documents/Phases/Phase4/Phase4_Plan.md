# Phase 4 Plan - Completing the Flight Loadout
*2026-04-20*

## Goal
Finish what Phase 3 started: every generated ship gets a complete set of flight systems (Short + Medium + Far), and the propellant layer gains enough substance to support a future "gather materials -> refine fuel" gameplay loop.

Phase 4 does **not** introduce damage, repair, refueling UX, or any new ship subsystem (scanners / probes / OS). It is strictly: round out the flight factory and make mixtures real enough that downstream phases have something to consume.

---

## 1. Three New Archetypes

One per remaining slot, plus one more. Concrete archetype choices are deferred — we'll pick them together in a follow-up pass. The plan below constrains the *shape* of the work, not the content.

### What "one per slot" means
- **Short** already has `RCSLiquidChemical`. Phase 4 adds one more Short archetype to give short-range rolls some variety.
- **Medium** has no archetype yet. Phase 4 adds one.
- **Far** has no archetype yet. Phase 4 adds one.

### Category boundaries
- Medium may reuse the existing `LiquidChemicalArchetype` scaffolding if the chosen archetype is a larger liquid-chemical engine (e.g. OMS-class, booster-class). That's mostly a values file.
- Far almost certainly requires a **new category subpackage** under `flight/` (e.g. `flight/nls_*.go`). NLS drives don't share liquid-chemical's field set — they need their own archetype struct, generator DAG, and instance type. The `TE_Flight.md` note about `TopSpeedFractionC` as the primary dial applies.
- If the second Short archetype is non-liquid (e.g. cold gas), same rule: new subpackage.

Picking archetypes happens in a separate conversation. When we do, each gets:
- A one-line physical premise
- Slot assignment
- Whether it reuses an existing category or needs a new one
- Propellant needs (drives changes to the mixtures layer below)

### Infrastructure that has to land alongside the archetypes
- **Provenance-consistency bias.** Deferred from Phase 3 because it was untestable with only one slot. With 3+ slots populated, the generator should bias manufacturer selection across slots toward the same civilization. Implement in Phase 4 so "salvaged vs. single-program" emerges naturally from the rolls.
- **Rarity weights** on archetype selection within a slot (currently uniform). Not strictly required, but cheap once we have more than one archetype per slot and worth doing while we're there.
- **Ship-level validation** tests — assert every generated ship has a non-null entry in all three flight slots, across a seed sweep.

---

## 2. Mixtures - Resource Composition

### Why
Today a `Mixture` is a bundle of physical properties (ISP multiplier, density, storability, cryogenic/hypergolic flags). It describes how the fuel *behaves*, but says nothing about where it *comes from*. Downstream gameplay (gathering, refining, trading) needs an answer to: "if I'm out of LOX/LH2, what do I have to find to make more?"

Phase 4 grows the mixture model to include that answer. Gameplay using it lands in a later phase.

### Shape of the change
Every `Mixture` gains a **recipe**: a list of resource inputs with quantities, plus any constraints on the refining process. The plan doesn't fix the exact field names — that's a small design call — but the content should capture:

- **Input resources.** Each entry is a pairing of `(ResourceID, QuantityPerUnitFuel)`. `QuantityPerUnitFuel` is expressed per kg of finished propellant so bookkeeping is consistent across mixtures.
- **Byproducts or waste, if relevant.** Some refining paths produce useful secondary outputs (oxygen during RP-1 cracking, etc.). Not strictly required for Phase 4 but the model should leave room.
- **Process constraints.** Cryogenic mixtures imply the need for refrigeration hardware to store the output. Hypergolic mixtures imply hazard-handling constraints. These already exist as flags on `Mixture` — the recipe should not duplicate them, but refining code later will read both.

A new registry (`resources.go`, sibling to `mixtures.go`) defines `Resource` with an `ID`, a human-readable name, and whatever categorization the later gathering loop will need (probably: phase-of-matter, commonality tier, typical source body type). Resource content is hand-authored, like civs/manufacturers/mixtures.

### Scope guardrails
- No refining *mechanic* in Phase 4. No UI, no time costs, no energy costs for the refining process. Just the data model: mixtures know what they're made of, resources exist as first-class objects.
- No gathering loop. Resources are defined but nothing produces or consumes them yet.
- No storage/tank modeling beyond what already exists. That's its own phase.
- Every existing mixture (`LOX_LH2`, `LOX_RP1`, `MMH_NTO`, `Hydrazine`) gets a recipe. New archetypes may pull in new mixtures, and those new mixtures also get recipes.

### Validation
- Every mixture's recipe references only registered resources. Add a package-init validation pass (same pattern as archetype validation) so a typo in a resource ID panics at server start, not at request time.
- Every mixture has at least one input resource. Empty recipes are always a mistake.

---

## 3. FactoryVersion

Both the archetype expansion and the mixture shape change are breaking: existing persisted loadouts may reference archetypes or mixture fields the new code doesn't know how to re-hydrate. Bump `FactoryVersion` when Phase 4 ships. Old ships stay readable (JSONB is permissive) but regeneration against a Phase 4 factory produces the new shape.

No migration of existing rows is in scope. If a player has a `phase3-v2` ship, it stays that way until regenerated via `/api/ships/generate`.

---

## 4. Out of Scope (Explicit)

Listing so we don't drift mid-phase:

- Damage / repair / break-type model (still future work from Phase 3 notes).
- Any client-side work (camera modes, HUD, controls).
- Scanners, probes, Spacecraft OS.
- Signal propagation, universe generation, spatial indexing.
- A refining UI, gathering loop, or resource-inventory model on the ship.
- Cross-category balance (power budget closure, thermal closure, delta-V targets) — still deferred until at least one non-flight category exists.

---

## 5. Open Questions for the Archetype Pass

To be answered in the follow-up conversation, not here:

- Which three archetypes (one per slot + one more, slot TBD)?
- Is the third archetype Short (variety) or a duplicate per slot?
- Does Far warrant one concrete drive type for Phase 4, or a stub that picks between a couple of sketched-in variants?
- Do any new archetypes introduce new mixtures, or do they all reuse the current four?
- Rarity weights: uniform across archetypes within a slot, or do we start authoring explicit weights now?
