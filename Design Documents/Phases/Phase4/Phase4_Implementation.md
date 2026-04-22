# Phase 4 Implementation — Relativistic Logistics
*2026-04-21*

## Purpose
Translate `Phase4_Plan.md` into a concrete, commit-by-commit implementation plan detailed enough that a developer (or AI agent) with no prior context can execute Phase 4 end-to-end. Do not duplicate design rationale that already lives in the Plan doc — reference it. This doc answers **what code lands in what order and why**.

**Phase 4 is infra-only.** No authored resources, no authored refinery archetypes, no new authored mixtures. Registries ship empty (or with retrofitted zero-values on the 4 existing mixtures); the user fills content in a post-infra pass.

---

## Prerequisites & starting state

**What exists today (branch `phase-4`, tip `527ad31`):**
- `server/internal/factory/mixtures.go` — 4 mixtures (`LOX_LH2`, `LOX_RP1`, `MMH_NTO`, `Hydrazine`). No `IgnitionNeed`, no `Synthetic`, no recipe.
- `server/internal/factory/flight/liquid_archetypes.go` — 8 authored archetypes. Only `RCAStandard`, `HPFAService`, `SCTAMainline` are registered in `init()`. The other five (`TCAShort`, `PDDPhotolytic`, `RDEShockwave`, `SABRE`) compile but never reach the generator, and several reference mixtures that don't exist (`HTP_90`, `Methalox`, `Aerozine50_NTO`, `Glass-Hydrazine`, `Methane_Fluorine`, `Hydrogen_Fluorine`, `Polyphosphate_Concentrate`, `CH3OH_Saline_Substrate`, `Hydrazine_Mono`, `Hydrolox`). Many are missing required fields.
- `server/internal/factory/flight/liquid.go` — archetype struct, `Validate()`, DAG generator.
- `server/internal/factory/flight/flight.go` — `GenerateForSlot` dispatcher, `ManufacturerPicker` injection, uniform archetype pick.
- `server/internal/factory/assembly/ship.go` — `FactoryVersion = "phase3-v2"`, `GenerateRandomShip` loops all three slots.
- `server/internal/factory/civilizations.go` — one `GenericCivilization` with `TechTier = 3`.

**Phase 4 lands:**
1. An empty **Resource** registry (types + helpers + validation; no authored resources).
2. `Mixture.IgnitionNeed *ResourceID` + `Mixture.Synthetic bool`. Existing mixtures compile with zero-value fields. Validation is permissive on unset ignition (only checked when non-nil).
3. A new **Refinery** subpackage with `RefineryArchetype`, `MixtureProduction`, and an empty registry. **No instances authored. No `ShipLoadout.Refinery` field yet.** Recipes live on productions so the same mixture can have multiple refinery-level production paths with different power / throughput / catalyst / wild-precursor inputs.
4. Sanity-check the 5 authored-but-unregistered liquid archetypes. Fill missing required fields. **Relax** the mixture-resolution validator to *warn* rather than panic so infra can land before content. Register archetypes whose mixtures resolve (the rest stay authored-but-unregistered and land when the user authors their mixtures).
5. New `Far` category subpackage with `RelativisticDriveArchetype` and a single registered `RBCA` archetype. `Far` slot gated to `TechTier >= 5`. `Matter_Antimatter_Pair` synthetic mixture.
6. Provenance-consistency bias and per-slot archetype rarity weights.
7. `FactoryVersion = "phase4-v1"`.
8. `Design Documents/TE_TimeDilation.md`. No runtime.

**Out-of-scope** (Plan §6):
- Damage / repair.
- Refinery runtime mechanics (wear, heat, dirty fuel, throughput simulation).
- **Refinery archetype content.** No instances authored. No `ShipLoadout.Refinery` field.
- **Resource / mixture / ignition content.** Registry empty.
- Time-dilation runtime.
- Refueling UX, gathering, inventory.
- Client work.

---

## Branch strategy

Already on `phase-4`, tip `527ad31`. Each commit below is a real commit on `phase-4`. Keep them small and atomic. Merge to `main` only after the post-phase verification passes.

---

## Commit-by-commit plan

### Commit 1 — `factory: Resource registry infrastructure (empty)`
**Intent.** Land the type system and registry for `Resource` so downstream commits (mixture `IgnitionNeed`, refinery recipes) have something to reference. Zero authored content — the user fills the registry later.

**Files created.**
- `server/internal/factory/resources.go`:
  - `type ResourceID string`.
  - `type ResourceCategory int` with `WildPrecursor`, `Catalyst`, `IgnitionComponent` constants + `String()`.
  - `type PhaseOfMatter int` with `Solid`, `Liquid`, `Gas`, `Plasma`, `Exotic` + `String()`.
  - `Resource` struct:
    ```
    ID                ResourceID
    DisplayName       string
    Category          ResourceCategory
    Phase             PhaseOfMatter
    Commonality       int      // 1..5
    TypicalSourceHint string
    ```
  - `var Resources = map[ResourceID]*Resource{}`.
  - `func registerResource(r *Resource)` — panics on duplicate ID or invalid fields (empty DisplayName, `Commonality` outside `[1,5]`).
  - `func LookupResource(id ResourceID) (*Resource, bool)`.
  - Empty `init()` with a comment: `// Resource registry starts empty — content authored in a post-infra pass.`

**Gotchas.**
- Do not author any resources. An empty registry is the intended end state for this commit.
- `ResourceID` is a named string type, not an alias (`type ResourceID string` rather than `type ResourceID = string`). Named types catch mix-ups at the type level — passing a `MixtureID` where a `ResourceID` is expected should not compile.

---

### Commit 2 — `factory: Mixture.IgnitionNeed + Synthetic + permissive validation`
**Intent.** Extend `Mixture` with the two new fields, retrofit the 4 existing mixtures with zero-values, and add permissive validation. No recipe on mixture — that lives on refinery productions (commit 3).

**Files modified.**
- `server/internal/factory/mixtures.go`:
  - Extend `Mixture`:
    - `IgnitionNeed *ResourceID` — nullable.
    - `Synthetic bool` — default false.
  - Retrofit the 4 existing mixtures: leave `IgnitionNeed = nil` and `Synthetic = false`. **Do not author recipes or ignition references** — user does that later.
  - Package-init validation:
    - If `IgnitionNeed != nil`: referenced resource must exist in `Resources` AND have `Category ∈ {IgnitionComponent, Catalyst}`. Panic on violation.
    - Validation is **empty-registry-safe**: mixtures with `IgnitionNeed == nil` pass regardless of whether `Resources` is empty.
    - Document but do NOT enforce the `IgnitionNeed==nil iff Hypergolic==true` invariant. Leave it as a `// TODO: enforce once content lands`. Enforcing now would trip on every existing mixture, since all have `IgnitionNeed == nil`.

**Gotchas.**
- `*ResourceID` (not `ResourceID` with empty-string sentinel) — nil/non-nil is semantic, not magic.
- Do NOT add a `Recipe` field to `Mixture`. Recipes live on refinery productions.
- Do NOT author `Matter_Antimatter_Pair`, `HTP_90`, etc. in this commit. Commit 5 adds `Matter_Antimatter_Pair` only because RBCA references it; everything else is user-authored post-infra.

---

### Commit 3 — `factory/refinery: subpackage + RefineryArchetype + MixtureProduction (empty registry)`
**Intent.** Stand up the refinery subpackage as a sibling to `flight/`, with the multi-level production model baked in. **No archetype instances. No `ShipLoadout.Refinery` field.** Validation runs on an empty registry and passes trivially.

**Files created.**
- `server/internal/factory/refinery/refinery.go`:
  - Imports `factory` for `ResourceID`, `ResourceInput`, `Resources`, `Mixtures`.
  - `type ResourceInput struct { ResourceID factory.ResourceID; QuantityPerUnitFuel float64 }` — live in `factory` package so both refinery and future systems can use it. (If not already there, add it to `factory/resources.go` in commit 1.)
  - `type MixtureProduction struct`:
    ```
    MixtureID             string
    Recipe                []factory.ResourceInput  // must bottom out at WildPrecursor resources
    CatalystID            factory.ResourceID
    CatalystUsePerKg      float64
    PowerDrawWRange       [2]float64
    ThroughputKgHourRange [2]float64
    ```
  - `type RefineryArchetype struct`:
    ```
    Name                 string
    Description          string
    TechTier             int         // 1..5
    HealthInitRange      [2]float64
    DryMassKgRange       [2]float64
    IdlePowerDrawWRange  [2]float64
    Productions          []MixtureProduction
    ```
  - `type Refinery struct` — embeds `factory.SystemBase`; rolled scalars `Health`, `DryMassKg`, `IdlePowerDrawW`, `Productions []MixtureProduction` (copied from archetype). Not produced in Phase 4 beyond the type definition.
  - `var registeredArchetypes []RefineryArchetype`.
  - `func registerRefineryArchetype(a RefineryArchetype)` — appends; panics on duplicate name.
  - `func (a RefineryArchetype) Validate() error`:
    - `Name` non-empty; `TechTier ∈ [1,5]`; all ranges well-formed.
    - `len(Productions) > 0` — only enforced when the archetype is actually being registered (empty registry skips this entirely).
    - For each production:
      - `MixtureID` resolves in `factory.Mixtures`.
      - Each `Recipe` entry's `ResourceID` resolves in `factory.Resources` with `Category == WildPrecursor`.
      - `CatalystID` resolves in `factory.Resources` with `Category == Catalyst`.
      - Ranges well-formed.
    - All resource/mixture checks are **empty-registry-safe** via `LookupResource` / `LookupMixture` — but since there are no registered archetypes yet, nothing is validated.
  - `func Archetypes() []RefineryArchetype { return registeredArchetypes }` — read-only accessor for consumers.
  - `init()`:
    ```
    // Refinery archetype registry starts empty — content authored in a post-infra pass.
    for _, a := range registeredArchetypes {
        if err := a.Validate(); err != nil { panic(...) }
    }
    ```
    The loop is a no-op today but wired so content additions Just Work.

**Files NOT modified (explicitly).**
- `server/internal/factory/assembly/ship.go` — `ShipLoadout` stays as-is. No `Refinery` field. Add a `// TODO(phase4-content): wire refinery into ShipLoadout once refinery archetypes are authored.` comment near the flight-slot loop.

**Gotchas.**
- Recipes live on `MixtureProduction`, not on `Mixture`. The same mixture can be produced by multiple refineries with *different* recipes, catalysts, and power draws — that's the whole point of the multi-level design.
- `ResourceInput` in `factory` package (not `refinery`) so future non-refinery consumers (e.g. engine catalyst rates) don't import `refinery`.
- No `SupportedMixtures` slice — `Productions` *is* the supported-mixtures list. A future helper `func (a RefineryArchetype) SupportedMixtureIDs() []string` derives it.
- Do not author any archetypes. `GenericChemicalRefinery` does not land in Phase 4.

---

### Commit 4 — `factory/flight: sanity-check authored liquid archetypes + relax mixture validator`
**Intent.** The 5 authored-but-unregistered archetypes are missing required fields and reference mixtures that don't exist. Fill the missing fields so they pass `LiquidChemicalArchetype.Validate()`. Relax the "mixture must resolve" check to a warning (log at package init) so infra can land before mixture content is authored. Register only the archetypes whose mixtures resolve in the current 4-mixture set.

**Files modified.**
- `server/internal/factory/flight/liquid.go`:
  - Locate the `Validate()` check that verifies each `AllowedMixtureIDs` entry resolves in `factory.Mixtures`. Change its behavior: if the entry does not resolve, log a warning (`log.Printf("flight: archetype %q references unauthored mixture %q — skipping registration", ...)`) and mark the archetype as unregisterable rather than panicking. Export a sentinel (`ErrMixtureNotAuthored`) so callers can distinguish.
  - Update `registerLiquidArchetype` to call `Validate()` up front; on `ErrMixtureNotAuthored`, log and *return without registering*. Other validation errors still panic.
- `server/internal/factory/flight/liquid_archetypes.go`:
  - **Fill missing required fields** on each authored-but-unregistered archetype. Exact list depends on what `Validate()` requires (`ReferencePressurePa`, `CountRange`, `IspAtRefPressureRange`, `DryMassRange`, `GimbalEligibleMassKg`, `GimbalRangeRange`, `OperatingPowerWRange`, `MaxContinuousBurnRange`, `MaxRestarts`, `MinThrottleRange`, `MaxThrottleRange`, `AblatorMassKgRange` if `Ablative ∈ AllowedCoolingMethods`). Values should be consistent with each archetype's premise — read the `Description` field.
  - **Apply mixture renames** where obvious:
    - `TCAShort`: `Hydrazine_Mono` → `Hydrazine`.
    - `SCTAMainline`: `Hydrolox` → `LOX_LH2`, `Methalox` → (keep, unresolved — will warn and skip).
  - **Register all 5** in `init()`. The relaxed validator skips those whose mixtures don't resolve; the user completes registration later by authoring mixtures.

**Gotchas.**
- Do not author new mixtures to make validation pass. The relaxed validator is the correct answer.
- Some archetypes will skip registration at startup. That's expected — log output should make it obvious which ones and why.
- `SCTAMainline` is currently registered. If the `Methalox` entry causes it to skip, `LOX_LH2` is enough to keep the archetype live — confirm `registerLiquidArchetype` treats partial mixture resolution as "register with whatever resolves" or as "skip entirely." Keep the semantics simple: archetype registers iff *at least one* mixture resolves. Log unresolved entries as warnings.

---

### Commit 5 — `factory/flight/far: Far subpackage + RBCA archetype + Matter_Antimatter_Pair`
**Intent.** Stand up the Far category and land the single `RBCA` archetype gated to tier-5 civs.

**Files created.**
- `server/internal/factory/flight/far.go`:
  - `type RelativisticDriveArchetype struct`:
    ```
    Name                 string
    Description          string
    FlightSlot           FlightSlot   // always Far
    TechTier             int          // min civ tier; RBCA = 5
    HealthInitRange      [2]float64
    TopSpeedFractionC    float64      // (0, 1) — primary gameplay dial
    IspVacuumRange       [2]float64
    ThrustNRange         [2]float64
    OperatingPowerWRange [2]float64
    AllowedMixtureIDs    []string
    SignatureProfile     string       // flavor only in Phase 4
    ```
  - `type RelativisticDrive struct` — embeds `factory.SystemBase`; rolled scalars mirror archetype.
  - `func (a RelativisticDriveArchetype) Validate() error` — ranges well-formed, `TopSpeedFractionC ∈ (0,1)`, `TechTier ∈ [1,5]`, `AllowedMixtureIDs` resolve (uses same relaxed rule as commit 4).
  - `func generateRelativisticDrive(...)` — uniform roll per scalar, uniform mixture pick, manufacturer via injected picker. Same DAG shape as liquid but simpler.
  - `init()` calls the flight package's `registerForSlot(Far, archetypeName, generator, techTier)` (see commit 6 for `techTier` on `archetypeEntry`) once `RBCABeamCore` is registered.
- `server/internal/factory/flight/far_archetypes.go`:
  - `var RBCABeamCore = RelativisticDriveArchetype{...}` per Plan §1:
    - `TopSpeedFractionC: 0.85`
    - `IspVacuumRange: [1_000_000, 5_000_000]`
    - `ThrustNRange: [500, 2_000]`
    - `OperatingPowerWRange: [10_000, 50_000]`
    - `AllowedMixtureIDs: ["Matter_Antimatter_Pair"]`
    - `HealthInitRange: [0.40, 0.60]`
    - `TechTier: 5`
    - `SignatureProfile: "gamma burst, omnidirectional — detectable across light-years"`
  - Register in `init()`.

**Files modified.**
- `server/internal/factory/mixtures.go` — add `Matter_Antimatter_Pair`:
  - `Synthetic: true`
  - `Cryogenic: true`, `Hypergolic: false`
  - `StorabilityDays: 0`
  - `IgnitionNeed: nil` — leave nil in Phase 4 (user authors `Magnetic_Trap_Assembly` resource + sets `IgnitionNeed` pointer in the content pass). Add a `// TODO(phase4-content): IgnitionNeed = &Magnetic_Trap_Assembly once resource is authored.` comment.

**Gotchas.**
- `Matter_Antimatter_Pair` is the first `Synthetic: true` mixture. Commit 2's permissive validator must let it through with no complaints.
- If RBCA's `AllowedMixtureIDs` fails to resolve (`Matter_Antimatter_Pair` not authored yet), the relaxed validator would refuse to register it — which defeats the purpose of Phase 4. Authoring `Matter_Antimatter_Pair` in this commit is the fix.
- `SignatureProfile` is flavor-only. Leave as string; scanner system later restructures.

---

### Commit 6 — `factory/flight: TechTier gating on Far slot`
**Intent.** Teach the slot dispatcher to filter archetypes by civ `TechTier`. Tier-<5 civs roll no Far archetype → slot is null.

**Files modified.**
- `server/internal/factory/flight/flight.go`:
  - Add `techTier int` to `archetypeEntry` (default 0 = no gate).
  - Change `registerForSlot` (or add `registerForSlotTiered`) to accept and store tier.
  - Add injection: `type CivTechTierLookup func(civID string) (int, bool)` + `SetCivTechTierLookup(fn)` + package var. Mirror the `ManufacturerPicker` wiring.
  - In `GenerateForSlot`: after fetching entries, resolve civ tier, filter to `entry.techTier <= civTier`. Empty filtered list → `ErrSlotEmpty` (existing return path; Medium/Far already serialize as JSON `null`).
- `server/internal/factory/flight/far.go`:
  - Register RBCA with `techTier: 5`.
- `server/internal/factory/flight/liquid.go`:
  - Register all liquid archetypes with `techTier: 0` (ungated).
- `server/internal/factory/assembly/ship.go` or wherever factory is initialized:
  - At startup wire the lookup: `flight.SetCivTechTierLookup(func(id string) (int, bool) { c, ok := factory.Civilizations[id]; if !ok { return 0, false }; return c.TechTier, true })`.

**Gotchas.**
- If the lookup isn't wired, treat as fatal (panic in dispatcher), mirroring `manufacturerPicker == nil`.
- The only registered civ is tier-3 `GenericCivilization`, so every generated ship in Phase 4 has `Flight[Far] == nil`. That's correct and the sweep test (commit 9) asserts it.

---

### Commit 7 — `factory: provenance-consistency bias`
**Intent.** Across slots in a single ship, bias manufacturer selection toward the manufacturer already picked for a previous slot (if that manufacturer is in the civ's roster). Deferred from Phase 3 — now testable with 2+ slots populated per ship.

**Files modified.**
- `server/internal/factory/manufacturers.go`:
  - Extend `PickManufacturerForCivilization` (or add a sibling) to take a `previousManufacturerID string` hint. Multiply its weight by `SameManufacturerBias = 3.0` if the previous manufacturer is in the civ's roster for this archetype; otherwise unchanged.
- `server/internal/factory/flight/flight.go`:
  - Extend `ManufacturerPicker` signature to `func(civID, archetypeName, previousMfgID string, rng *rand.Rand) (string, error)` (or add a new method). Propagate.
  - Thread `previousMfgID` through `GenerateForSlot`.
- `server/internal/factory/assembly/ship.go`:
  - Extract the previously-chosen manufacturer ID from the returned flight system between slot rolls. Requires a `ManufacturerID() string` method on whatever `GenerateForSlot` returns. Both `LiquidChemicalEngine` and `RelativisticDrive` embed `factory.SystemBase` which already carries `ManufacturerID` — add the method to `SystemBase` once and both inherit.
  - Thread `previousMfg` through the slot loop.

**Gotchas.**
- Bias is soft — if the previous manufacturer has zero weight for the new archetype, pick freely. Do not force.
- `FlightSystem` is an empty interface today. Adding a single-method interface (`type ManufacturerIdentifiable interface { ManufacturerID() string }`) is the cleanest route.

---

### Commit 8 — `factory/flight: rarity weights on archetype selection`
**Intent.** Replace uniform archetype pick within a slot with weighted sampling. Common archetypes dominate; exotic archetypes are rare.

**Files modified.**
- `server/internal/factory/flight/flight.go`:
  - Add `weight float64` to `archetypeEntry` (default 1.0).
  - Extend registration API to accept weight. Use a struct-arg pattern if the positional list is getting unwieldy: `registerForSlot(RegisterOpts{Slot, Name, Generator, TechTier, Weight})`.
  - In `GenerateForSlot`, after tier filtering, weighted-sample using an existing helper in `factory/` or add `weightedPick[T]` locally.
- `server/internal/factory/flight/liquid_archetypes.go` and `far_archetypes.go`:
  - Author weights in `init()`:
    - Short: `RCAStandard = 3.0`, `TCAShort = 2.0`, `PDDPhotolytic = 0.5`.
    - Medium: `HPFAService = 3.0`, `SCTAMainline = 2.0`, `RDEShockwave = 0.8`, `SABRE = 0.3`.
    - Far: `RBCABeamCore = 1.0`.

**Gotchas.**
- Weight 0 is legal (temporary disable).
- Do not confuse this with `Commonality` on resources — different system.

---

### Commit 9 — `assembly: FactoryVersion bump + seed-sweep ship shape test`
**Intent.** Bump the version tag and add a tripwire test that keeps Phase 4's invariants locked in.

**Files modified.**
- `server/internal/factory/assembly/ship.go` — `const FactoryVersion = "phase4-v1"`.

**Files created.**
- `server/internal/factory/assembly/ship_sweep_test.go`:
  - `TestShipShape_SeedSweep` — 1000 seeds:
    - `Flight[Short]` non-nil on every ship.
    - `Flight[Medium]` non-nil on every ship.
    - `Flight[Far] == nil` on every ship (tier-3 civ is the only registered civ; assert explicitly so a future tier-5 civ registration trips this test as a reminder to revisit).
    - `ShipLoadout.Refinery` does **not** exist yet — the struct must not expose a refinery field. (Add a comment in the test pointing this out so the next phase doesn't forget.)
    - Every non-nil flight system: mixture resolves in `factory.Mixtures`; manufacturer ID non-empty.
  - `TestFarUnlocksForTier5` — inject a synthetic tier-5 civ into `factory.Civilizations` in `TestMain`, confirm `Flight[Far]` is non-nil when that civ is chosen, remove on teardown.

**Gotchas.**
- Sweep test is strict by design. Make its assertions specific.
- The `Flight[Far] == nil` assertion is a tripwire for the first tier-5 civ.

---

### Commit 10 — `docs: TE_TimeDilation.md`
**Intent.** Commit the design doc Plan §4 references. No code.

**Files created.**
- `Design Documents/TE_TimeDilation.md`:
  - Header + `Status: Design Only (Phase 4 documents; future phase implements)`.
  - Sections (paragraph each):
    1. **Objective.** Shared universal clock + per-ship proper time; diverge at relativistic speeds.
    2. **Lorentz transformation.** γ = 1/√(1−v²/c²); `Δt_ship = Δt_global / γ`; SI units throughout.
    3. **Clock identities.** `GlobalEpoch` monotonic, server-owned. `ProperTime` per-ship, cumulative, starts at 0.
    4. **Decay routing.** Ticks against `ProperTime`: engineering wear, battery drain, catalyst consumption, life-support, biological aging. Ticks against `GlobalEpoch`: probes, beacons, derelict wrecks, signal propagation, resource depletion on bodies, civ-level political state.
    5. **Far transit session flow.** Player logs off, universe runs, ship physically traverses, player returns to changed world. Not teleportation.
    6. **Out of scope for this doc.** Session flow commit/arrival events; client-side dilation effects; aging UI; inter-ship clock reconciliation (hard problem, flagged).
  - No code, no field signatures.

**Gotchas.**
- Reference material; keep short. No runtime stubs.

---

## Post-phase verification (manual)

1. Fresh DB.
2. `go test ./server/...` — all green.
3. Start server. `curl -X POST "http://localhost:8080/api/ships/generate?player_id=<id>&seed=1"`:
   - `flight.short` populated, `flight.medium` populated (if registered archetypes survived commit 4's relaxed validator), `flight.far` is `null`.
   - No `refinery` key on the ship JSON (deferred).
   - `factory_version == "phase4-v1"`.
4. Repeat with `seed=1` several times — bit-identical loadouts.
5. Inspect `grep -r "Resources\[" server/` — no authored resources beyond what the user adds manually.
6. Confirm `Design Documents/TE_TimeDilation.md` exists.
7. Confirm no `ProperTime` / `GlobalEpoch` / γ symbols exist in Go.

---

## Deferred-but-spec'd items (extension points)

- Resource / mixture / ignition / refinery content authoring — user's post-infra pass.
- `ShipLoadout.Refinery` field + per-ship refinery roll — lands when at least one refinery archetype exists.
- Coverage invariant test (every non-synthetic mixture has a refinery production) — deferred until content exists.
- `RelativisticDrive.SignatureProfile` → structured schema (scanner phase).
- Refinery runtime (catalyst wear, heat, dirty fuel, throughput ticks).
- Time-dilation runtime.
- Far-slot archetypes for tiers 1–4.
- Civ-tiered refinery variants.

---

## Summary of what lands

An empty resource registry with the full type system; `Mixture.IgnitionNeed` + `Mixture.Synthetic` retrofitted with zero-values; a new `refinery` subpackage whose `RefineryArchetype.Productions` supports multi-level production of the same mixture (empty registry for now, no `ShipLoadout.Refinery` field yet); the 5 authored-but-unregistered liquid archetypes sanity-checked and registered (mixture validator relaxed to warn rather than panic so content can lag infra); a Far category subpackage with `RBCA` gated to tier-5 civs; `Matter_Antimatter_Pair` authored so RBCA has a mixture to reference; provenance bias + rarity weights on archetype selection; `FactoryVersion = "phase4-v1"`; `TE_TimeDilation.md` in design docs. No authored content beyond what's strictly required to keep the type system live. No runtime mechanics.
