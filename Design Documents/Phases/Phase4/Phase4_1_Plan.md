# Phase 4.1 Plan — Metabolic Mixtures
*2026-04-21*

## Goal
Make `Mixture` the authoritative object for a propellant's **cost of existence** — the precursors it consumes, the power and time it takes to synthesise, the catalyst it wears out, the thing that has to be alive to light it. Collapse Phase 4's refinery-owned `MixtureProduction` recipe into the mixture itself, and demote the `Refinery` to a *modulator* (efficiency, throughput cap, catalyst wear, heat) rather than a recipe owner.

Phase 4.1 ships the **data model and validation**. It does not ship the refining loop, the HUD, the gathering loop, or the time-dilation runtime. Content — specific mixtures, specific precursors, specific catalysts — is still authored in a post-infra pass. The goal here is to land the right shape so that pass can happen in one sitting without revisiting the schema.

Rename cue: "metabolic mixtures" because a mixture now declares what the ship has to *eat and digest* to produce it, not just what comes out the nozzle.

---

## 1. The Shape Change

### What Phase 4 shipped

Phase 4 put the recipe on the refinery side:

```go
// refinery.MixtureProduction — Phase 4
type MixtureProduction struct {
    MixtureID             string
    Recipe                []factory.ResourceInput
    CatalystID            factory.ResourceID
    CatalystUsePerKg      float64
    PowerDrawWRange       [2]float64
    ThroughputKgHourRange [2]float64
}
```

The premise was multi-path production: one mixture could be produced by multiple refinery archetypes with different recipes, different catalysts, different throughput envelopes. That premise is discarded in 4.1. Reasons:

- **Player model.** A player reasoning about a mixture reasons about *one* set of precursors. "To make Hydrazine I need ammonia" is the interesting sentence; "to make Hydrazine I need ammonia on Refinery A but hydrocarbons on Refinery B" is a sentence nobody will ever internalise. The multi-path model is expressive in the schema and invisible in play.
- **Authoring cost.** Multi-path requires authoring each production pairwise, n_mixtures × n_refineries, and every new refinery archetype forces a content audit across every mixture it might plausibly support. One recipe per mixture authors linearly.
- **Refineries still differentiate.** Efficiency, throughput cap, catalyst tolerance, and heat output are enough levers to make two refineries feel different without also duplicating chemistry. A budget refinery wastes 30% of the feedstock; a premium one runs at 98% yield with a tighter thermal envelope. That's the friction surface.

### What Phase 4.1 makes canonical

```go
package factory

import "time"

type Mixture struct {
    ID             string
    Description    string
    Config         PropellantConfig
    IspMultiplier  float64
    DensityKgM3    float64

    // Synthesis — the "metabolic" layer. Canonical and single-path.
    Precursors        []ResourceInput  // wild-precursor inputs per kg output
    PowerCostPerKg    float64          // watts required during refining
    RefiningTimePerKg time.Duration    // ship-time per kg (τ, see §3)
    RequiredCatalyst  ResourceID       // hardware consumable; wears down

    // Operational constraints
    IsCryogenic      bool
    IsHypergolic     bool
    IgnitionResource *ResourceID       // nil iff IsHypergolic
    Synthetic        bool              // bypasses refineries entirely

    // Storability (carry-over from Phase 4; orthogonal to synthesis)
    StorabilityDays int // -1 = indefinite
}
```

`Refinery` shrinks to modulation + wear state:

```go
package refinery

type RefineryArchetype struct {
    Name                string
    Description         string
    TechTier            int
    HealthInitRange     [2]float64
    DryMassKgRange      [2]float64
    IdlePowerDrawWRange [2]float64

    // Modulation, not recipe ownership.
    EfficiencyRange      [2]float64 // 0..1 yield multiplier on Mixture.Precursors
    ThroughputLimitRange [2]float64 // hard kg/hr cap regardless of mixture
    HeatOutputPerWRange  [2]float64 // thermal cost per watt drawn

    // Gating — which mixtures this refinery can even attempt. A mixture
    // the refinery doesn't list is refused, even if the ship has the
    // precursors. Set at archetype-author time; no recipe here.
    SupportedMixtureIDs []string
}

type Refinery struct {
    factory.SystemBase
    Health           []float64
    DryMassKg        float64
    IdlePowerDrawW   float64
    Efficiency       float64 // rolled once from EfficiencyRange
    ThroughputLimit  float64 // rolled once from ThroughputLimitRange
    HeatOutputPerW   float64
    CatalystHealth   float64 // 0..1, wears with use (runtime, later phase)
    SupportedMixtureIDs []string
}
```

`MixtureProduction` goes away. Anywhere it was used, fall through to `Mixture` for the recipe and `Refinery` for the modulation.

### Field-by-field rationale

- **`Precursors []ResourceInput`** — wild-precursor inputs only, bottoming out at `WildPrecursor`-category resources. The validator enforces this (carried forward from Phase 4's refinery validator).
- **`PowerCostPerKg float64`** — continuous watts drawn while a kg is being processed. Scalar, not a range: the range is the refinery's business (efficiency, idle draw). The mixture declares the *chemistry's* demand.
- **`RefiningTimePerKg time.Duration`** — the hook for time dilation (§3). Duration type, not a float, so call sites type-check the clock-frame choice.
- **`RequiredCatalyst ResourceID`** — hardware consumable. Distinct from `Precursors`: a catalyst is *worn*, not *consumed into the product*. Enforced via resource category (`Catalyst`). Empty string means catalyst-free.
- **`IgnitionResource *ResourceID`** — kept as a pointer to preserve the Phase 4 invariant: `IgnitionResource == nil iff IsHypergolic`. Phase 4.1 *tightens* this invariant to reject mismatches at registration time (Phase 4 was permissive; see §4).
- **`Synthetic bool`** — carry-over. A synthetic mixture has empty `Precursors`, zero `PowerCostPerKg`, zero `RefiningTimePerKg`, no `RequiredCatalyst`. Matter/Antimatter stays synthetic. Validator treats synthetic as a complete bypass of the metabolic fields.

### What the Refinery *earns* in the trade

By giving up recipe ownership, the refinery gets richer modulation and clearer gameplay identity:

- **Efficiency** — multiplies actual precursor consumption. A 0.75-efficiency refinery burns 33% more ammonia than the recipe says to produce the same 1 kg of hydrazine.
- **ThroughputLimit** — a hard ceiling independent of mixture. A small refinery can't saturate a big engine even given infinite feedstock.
- **CatalystHealth** — wears down with use; below some threshold the refinery produces "dirty fuel" (future phase: an Isp penalty on the resulting propellant). The catalyst *identity* comes from the mixture; the *wear state* lives on the refinery.
- **HeatOutputPerW** — couples the refinery to future thermal-management UX. Not consumed in 4.1.
- **SupportedMixtureIDs** — gating, not recipes. The refinery declares which chemistries it can run; if a mixture is on the list, the refinery uses *the mixture's* recipe, modulated by its own efficiency.

---

## 2. Migration from Phase 4

### What moves where

| Phase 4 location                              | Phase 4.1 location                          |
|-----------------------------------------------|---------------------------------------------|
| `refinery.MixtureProduction.Recipe`           | `factory.Mixture.Precursors`                |
| `refinery.MixtureProduction.CatalystID`       | `factory.Mixture.RequiredCatalyst`          |
| `refinery.MixtureProduction.CatalystUsePerKg` | *(dropped — catalyst wear lives on Refinery.CatalystHealth runtime state)* |
| `refinery.MixtureProduction.PowerDrawWRange`  | `factory.Mixture.PowerCostPerKg` (scalar — range collapses; refinery's efficiency modulates) |
| `refinery.MixtureProduction.ThroughputKgHourRange` | `refinery.RefineryArchetype.ThroughputLimitRange` (moved onto refinery — not per-mixture) |
| —                                             | `factory.Mixture.RefiningTimePerKg` **(new)** |
| —                                             | `refinery.RefineryArchetype.EfficiencyRange` **(new)** |
| —                                             | `refinery.RefineryArchetype.HeatOutputPerWRange` **(new)** |
| `refinery.MixtureProduction` (type)           | **deleted** |

Phase 4 shipped the refinery registry *empty*. No content has to be ported — only the schema. `Mixtures` registry ports forward with the existing five entries (LOX_LH2, LOX_RP1, MMH_NTO, Hydrazine, Matter_Antimatter_Pair); every entry gets synthesis fields zero-valued so the post-infra content pass fills them in.

### Carry-forward invariants

- **Permissive on empty content.** Unset `Precursors` is legal; so is unset `RequiredCatalyst`, zero `PowerCostPerKg`, zero `RefiningTimePerKg`. Validation only fires on *present* fields. This is the same warn-and-skip / permissive-on-empty discipline that let Phase 4 infra land before content.
- **Synthetic mixtures bypass all metabolic fields.** Validator short-circuits on `Synthetic == true` — none of the synthesis fields are required, and setting any of them is a validation error (inconsistent — either it's synthetic or it has a recipe).
- **Ignition dual-invariant tightens.** Phase 4 allowed `IgnitionResource == nil` regardless of `IsHypergolic`, because no content had been authored. Phase 4.1 enforces: `IgnitionResource == nil` iff `IsHypergolic == true`. Existing Phase 4 entries that violate this get either an authored `IgnitionResource` or a `IsHypergolic: true` flag in the same commit that lands the tightened validator.
- **`MixtureID` on engines still resolves via `LookupMixture`.** No engine-side change; the flight package continues to reference mixtures by ID and doesn't care that the internal shape grew.

---

## 3. Coupling to Time Dilation

`RefiningTimePerKg` is the single field that makes `TE_TimeDilation.md` *mean something gameplay-wise* in Phase 4.1. The rule:

**Refining ticks on proper time (τ, ship clock).**

A 15-minute-per-kg hydrazine run started while the ship is burning a Far drive at 0.85c (γ ≈ 1.9) completes 15 minutes later on the ship clock — but ~28 minutes have elapsed in the shared coordinate frame. When the ship arrives somewhere, the refinery's output is available, but the market the ship is delivering to has moved.

This is the first place in the codebase where "τ vs t" is not just a design note. Phase 4.1 does **not** implement the tick — the runtime loop is still future work — but the *type* makes the clock-frame choice explicit:

- `RefiningTimePerKg` is a `time.Duration` specifically so future runtime code has to choose between a `τ`-aware tick loop and a `t`-aware one at the call site. No silent float arithmetic against an ambient clock.
- A comment on the field documents the frame: *"processed against proper time (τ). See TE_TimeDilation.md."*

That's all 4.1 owes the clock model. The loop itself lands with the runtime phase.

---

## 4. Validation

All validation is at package-init, empty-registry-safe, and permissive on unset fields (consistent with Phase 4 discipline).

### `factory.Mixture` validator

For each registered mixture:

1. **Synthetic short-circuit.** If `Synthetic == true`, assert `len(Precursors) == 0`, `PowerCostPerKg == 0`, `RefiningTimePerKg == 0`, `RequiredCatalyst == ""`. Any non-zero synthesis field on a synthetic mixture is a panic — inconsistent declaration.
2. **Precursor category.** Every entry in `Precursors` must resolve to a `WildPrecursor`-category resource. `QuantityPerUnitFuel > 0`. Empty `Precursors` is legal (content pass fills it in); partially authored is not — either fully empty or all valid.
3. **Catalyst category.** If `RequiredCatalyst != ""`, it must resolve to a `Catalyst`-category resource.
4. **Ignition dual-invariant.** Assert `(IgnitionResource == nil) == IsHypergolic`. If `IgnitionResource != nil`, it must resolve and have category `IgnitionComponent` or `Catalyst`.
5. **Power / time monotonicity.** If `PowerCostPerKg > 0` then `RefiningTimePerKg > 0` and vice versa. A mixture that claims power but no time (or time but no power) is nonsense.

### `refinery.RefineryArchetype` validator

1. All existing Phase 4 checks (Name non-empty, TechTier ∈ [1,5], Health / DryMass / IdlePower range sanity) carry forward.
2. `EfficiencyRange` ⊂ (0, 1]. Zero efficiency would mean infinite feedstock for any output; 1.0 is the theoretical ceiling.
3. `ThroughputLimitRange[0] > 0`, `[1] >= [0]`.
4. `HeatOutputPerWRange[0] >= 0`, `[1] >= [0]`.
5. Every entry in `SupportedMixtureIDs` must resolve via `LookupMixture`. Empty list is legal at schema-validation time (archetype with no supported mixtures is useless but not invalid); registration-time check (separate from `Validate`) enforces non-empty the same way Phase 4 enforced non-empty `Productions`.

### Cross-package validator (new)

Runs after both packages' inits complete. For every registered refinery archetype:

- Every `SupportedMixtureIDs` entry must point at a mixture whose `Synthetic == false`. Synthetic mixtures bypass refineries; listing one is a declaration error.
- Every non-synthetic mixture with non-empty `Precursors` must be supported by at least one registered refinery archetype, **warn-only** (logged, not panicked). This is the "is anything actually reachable" check. Warn-only because Phase 4.1 lands before content — no refinery archetypes are registered yet.

---

## 5. Out of Scope

The following are explicitly *not* Phase 4.1:

- **Refining runtime loop.** No tick code. No kg-per-hour accumulation. No catalyst-health decay.
- **Gathering loop.** No wild-precursor harvesting. No asteroid / comet interaction.
- **Refueling UX.** No pipe-from-tank-to-engine flow.
- **Dirty-fuel Isp penalty.** `CatalystHealth` is declared; no engine reads it yet.
- **Content.** No authored precursors, catalysts, ignitions, refinery archetypes, or recipes land in this phase. The Phase 4 mixture registry ports forward with synthesis fields zero-valued. A separate content pass (authored by the user) fills them in.
- **Migration of existing saves.** Phase 4.1 bumps `FactoryVersion` (see §7). Ships generated under `phase4-v1` are not migrated — a later repair phase will own that.

---

## 6. Implementation Commits

Targeted as a short sequence — the architectural work is schema-shaped, not algorithmic.

1. **`factory: add metabolic synthesis fields to Mixture`** — extend `factory.Mixture` with `Precursors`, `PowerCostPerKg`, `RefiningTimePerKg`, `RequiredCatalyst`. All zero-valued on existing entries. No validator changes yet.
2. **`factory: tighten Mixture validator for synthesis fields`** — land §4 rules 1–5 on `factory.Mixture`. Tighten the ignition dual-invariant. Existing entries may need an `IsHypergolic` or `IgnitionResource` fix in this commit.
3. **`refinery: drop MixtureProduction, promote modulation fields`** — delete `MixtureProduction`; add `EfficiencyRange`, `ThroughputLimitRange`, `HeatOutputPerWRange`, `SupportedMixtureIDs` on the archetype; add `Efficiency`, `ThroughputLimit`, `HeatOutputPerW`, `CatalystHealth`, `SupportedMixtureIDs` on the instance. Registry stays empty.
4. **`refinery: validator for modulation + cross-package reachability check`** — land §4 rules on the archetype; land the cross-package warn-only reachability check.
5. **`factory: bump FactoryVersion to phase4_1-v1`** — stamp the new shape. Seed-sweep test gets the new version assertion.
6. **`docs(phase4.1): close out — mark content pass as next`** — one-line note in the Phase 4 directory pointing at the content pass. Not a full plan; a content pass is authoring, not engineering.

Six commits, each independently testable, each landing green.

---

## 7. Open Questions

- **Should `RefiningTimePerKg` scale with refinery efficiency?** Current draft: no. The refinery's efficiency modulates *feedstock consumption* (kg of precursor per kg of output), not *time*. Time is the mixture's chemistry. Argument for coupling: a worse refinery probably runs slower too. Argument against: it collapses two dials into one, and throughput (kg/hr cap) already captures "refinery is slow." Leave independent for 4.1, revisit if the content pass finds the distinction invisible.
- **Should catalysts be consumed or worn?** Phase 4 had `CatalystUsePerKg` (consumption per unit output). Phase 4.1 moves to *wear* (`CatalystHealth` on the refinery, decays with use). Consumption is crisper to simulate but means every refining run is a mining run; wear feels more like "stewardship of the refinery itself" (which is what the metabolic framing is about). Going with wear; flag here so we can revisit if wear-without-replacement leads to dead-end failure states.
- **What happens when a ship has a refinery but no compatible mixture in its engine's `AllowedMixtureIDs`?** This is a ship-generation validation concern: at roll time the picker has to guarantee at least one engine-mixture-refinery triangle closes. Out of scope for 4.1 (refinery isn't on `ShipLoadout` yet), but the cross-package reachability check is the hook that future ship-level validation will extend.
- **Mixture variants from the same precursors.** The old multi-path model made it cheap to author "cheap Hydrazine" and "premium Hydrazine" as two productions of the same mixture. In 4.1 that becomes two separate mixtures (`Hydrazine_Crude`, `Hydrazine_Premium`). If the content pass wants this and finds it awkward, revisit — but don't pre-bake it before anyone asks.

---

## 8. Acceptance

Phase 4.1 is done when:

- `factory.Mixture` carries the full metabolic field set.
- `refinery.MixtureProduction` is removed; refineries modulate rather than own recipes.
- Validators enforce §4 at package init, empty-registry-safe.
- Cross-package reachability check runs (warn-only) and produces no spurious warnings on an empty refinery registry.
- `FactoryVersion` bumped.
- All existing Phase 4 tests still pass; no new failures introduced by the schema change.
- The content-pass author can sit down, fill in `Precursors` / `PowerCostPerKg` / `RefiningTimePerKg` / `RequiredCatalyst` / `IgnitionResource` on the existing mixtures and author refinery archetypes, without touching any of the schema or validator code.
