# Phase 4 Plan - Relativistic Logistics
*2026-04-21*

## Goal
Finish the flight loadout (Short + Medium + Far on every generated ship), land the "metabolic" resource/refinery **infrastructure** so downstream gameplay has something to plug into, and commit a design doc for relativistic time dilation so Far drives have a coherent theoretical basis. No runtime time-dilation code, no refining mechanic, **no content authoring of specific resources / mixtures / recipes / ignitions** — those are left to the user in a post-infra content pass.

Phase 4 does **not** introduce damage/repair UX, refueling UX, the gathering loop, time-dilation *code*, refinery *runtime mechanics*, or any new ship subsystem beyond flight + refinery (no scanners / probes / OS). It is strictly: round out the flight factory, give mixtures and the refinery a real **data shape** with empty-content validation that lets the user fill in resources / recipes / ignitions later without revisiting the infra, and write down how the two-clock model will eventually work.

---

## 1. Archetype Completion

Every slot gets at least one archetype. Rolled ships can no longer be returned with empty slots (Tier-1..4 ships excepted on Far — see below).

### Far - RBCA (Relativistic Beam-Core Assembly)
The first and only Far archetype in Phase 4. Matter/antimatter annihilation in a magnetic nozzle; relativistic transit; intense gamma signature; fragile by construction.

- Slot: Far
- `TopSpeedFractionC`: ~0.85
- `IspVacuumRange`: 1,000,000 - 5,000,000 s
- `ThrustNRange`: 500 - 2,000 N (continuous, low-g)
- `OperatingPowerW`: 10,000 - 50,000 (magnetic containment dominates)
- Allowed mixtures: `Matter_Antimatter_Pair`
- `HealthInitRange`: 0.40 - 0.60 (spawns damaged — the drive *is* the adventure)
- **TechTier gate: 5 only.** Tier 1-4 civs roll no Far archetype in Phase 4. Ships from those civs are sublight-only by construction; their "Far slot" is null and the validator accepts that for sub-Tier-5 ships. This is deliberate, not a TODO.

Far needs a new category subpackage (`flight/far_*.go`). The field set diverges from liquid-chemical — relativistic drives need `TopSpeedFractionC`, power draw as a first-class field, and a signature profile stub (flavor only in Phase 4, no scanner consumes it yet).

### Medium and Short - review authored archetypes
Archetypes already drafted in `server/internal/factory/flight/liquid_archetypes.go` but not yet registered:

- **TCA (Thermal Catalytic Assembly)** — Short. Monopropellant catalyst-bed steam thruster. High reliability, low Isp.
- **PDD (Photolytic Decomposition Drive)** — Short. UV-laser dissociation; no combustion instability; heavy electrical draw.
- **HPFA (Hypergolic Pressure-Fed Assembly)** — Medium. Storable bipropellants, pressure-fed, no turbopump.
- **SCTA (Staged Combustion Turbopump Assembly)** — Medium. Mainline high-Isp staged-combustion engine, mechanically complex.
- **RDE (Rotating Detonation Manifold)** — Medium. Supersonic detonation ring; high T/W; vibration-limited.
- **SABRE (Synthetically Actuated Biogenic Reaction Engine)** — Medium. Biological / engineered-extremophile exhaust; low thermal signature; needs metabolic substrate.

Phase 4 does *not* author these — they exist. The work is:

- Sanity-check each authored archetype against the `LiquidChemicalArchetype` validation rules (ISP ordering, cooling method vs. chamber pressure, gimbal eligibility vs. dry mass, etc.). Fix values that trip validation.
- Sanity-check their propellant declarations against the mixtures registry — every `AllowedMixtureIDs` entry must resolve to a real mixture. Several currently reference mixtures that don't fit the 4-precursor model (fluorine-based, phosphorus-based) and will need rerouting or ejection. See §2 and §7.
- Register them in the `init()` block so the generator can actually roll them (currently only RCA / HPFA / SCTA are registered).
- Confirm the physical premise reads as plausible — push back on any archetype that's basically a reskinned RCA or duplicates an existing role.

### Infrastructure landing alongside
- **Provenance-consistency bias.** Deferred from Phase 3; now testable with 3+ populated slots. Generator biases manufacturer selection across slots toward the same civilization.
- **Rarity weights** on archetype selection within a slot (currently uniform). Author explicit weights now that slots have more than one option.
- **Ship-level validation tests.** For every seed in a sweep: Short and Medium slots non-null on every ship; Far slot non-null iff civ is Tier 5; Refinery non-null on every ship.

---

## 2. Resources & Mixtures — the Metabolic Model

### Why
Two gaps close here. First, a `Mixture` today says how fuel *behaves*, not where it *comes from* or what it takes to *light*. Second, there's no model for the loop that turns wild mass into usable propellant. Phase 4 lands the data shape for both; gameplay lands later.

### The metabolic model
Players don't find refined fuel in the void. They find **wild precursors** (ices, noble gases, ore) on asteroids and comets, feed them into a ship-borne **Refinery** (see §3), and the Refinery produces refined propellant for the engines to burn. Refined chemicals (LOX, LH2, MMH, hydrazine, etc.) are *intermediate states inside the refinery*, not first-class resources. **They never appear in the resource registry.**

### Resources
New file `server/internal/factory/resources.go`, sibling to `mixtures.go`. Hand-authored registry, same pattern as civs / manufacturers / mixtures. **Registry ships empty in Phase 4** — infra only. User authors content in a post-infra pass.

Three categories (enum on the resource itself):
- **Wild Precursors.** Raw mass found in the void (ices, noble gases, ore). Never produced by a refinery — they're the leaves of the recipe graph.
- **Catalysts.** Consumables that wear with use. A catalyst can live in a refinery *or* an engine; the role distinction is just which system references it. Single `Catalyst` enum value.
- **Ignition Components.** One-shot-ish hardware needed to light a burn (pyrotechnic charges, discharge capacitors, laser diodes, magnetic traps).

Each `Resource` carries `ID`, display name, category, phase-of-matter, commonality tier (1–5), and an optional source-body hint. Data-only in Phase 4 — nothing produces or consumes resources yet.

Refined chemicals (LOX, LH2, MMH, etc.) are **not** resources. They are mixtures, produced by refineries from wild precursors. They never appear in the resource registry.

### Mixture structure update
Every `Mixture` gains:

- **`IgnitionNeed *ResourceID`** — ignition component required to light this mixture. Nullable *iff* `Hypergolic == true`. Dual-direction invariant.
- **`Synthetic bool`** — flag for propellants without a refinery path (e.g. matter/antimatter). Synthetic mixtures bypass the refinery layer entirely.

Cryogenic / hypergolic flags stay where they are. **Recipes do *not* live on the mixture** — they live on the refinery, one per (refinery, mixture) pair (see §3). That's what lets one mixture have multiple production paths with different power / throughput / catalyst profiles.

### Scope guardrails
- No refining mechanic. No UI, no time/energy costs, no dirty-fuel state. Data model only.
- No gathering loop. Resources exist as first-class objects; nothing produces or consumes them.
- No storage / tank / inventory model. Ignition components are declared as a *requirement*, not inventoried.
- **No authored content.** Resource registry empty, no new mixtures authored, `IgnitionNeed` may be `nil` on any mixture until the user fills it in. Validation still runs but has nothing to reject.

### Validation (runs on whatever content exists, empty-registry-safe)
- If `IgnitionNeed != nil`, the referenced ID resolves in the resource registry and has `Category ∈ {IgnitionComponent, Catalyst}`.
- `IgnitionNeed == nil` iff `Hypergolic == true`. **Only enforced on mixtures where the user has set a value.** Phase 4 defers the dual-direction check on existing mixtures by gating it on a `RecipeAuthored` sentinel or by simply leaving it unenforced until content lands. The invariant is documented; the compiler is not a content-police yet.
- Registry lookups never panic on an empty registry.

---

## 3. The Refinery

### Why
Every ship needs a Refinery to turn wild mass into fuel. Phase 4 lands the data shape and plans for multi-level refineries — the same mixture can be produced by several different refineries with different power draws, recipes, and catalysts. Runtime mechanics (catalyst wear, heat, dirty fuel, breakdown risk) defer to a later phase. **No archetype instances authored in Phase 4.**

### Multi-level refinery model
A `RefineryArchetype` is a production plant. It carries a list of mixtures it can produce, and for **each** supported mixture it declares its *own* recipe, power profile, throughput, and catalyst behavior. So `MMH_NTO` might be produced by three different refineries — a crude tier-1 plant with a simple recipe and high power draw, a mainstream tier-3 plant, and a tier-5 plant with an exotic catalyst and tight throughput. The mixture itself is agnostic to how it's made.

This is the inversion of the obvious design (recipe-on-mixture). Putting recipes on the refinery is what makes the multi-level story possible.

### Shape of the change
New subpackage `server/internal/factory/refinery/`, parallel to `flight/`. Defines:

- **`MixtureProduction`** — one (refinery, mixture) pair:
  - `MixtureID string`
  - `Recipe []ResourceInput` — **wild-precursor** inputs, `QuantityPerUnitFuel` per kg of finished propellant. Must bottom out at wild precursors.
  - `CatalystID ResourceID` — catalyst this recipe consumes
  - `CatalystUsePerKg float64` — consumption rate (declared, not simulated)
  - `PowerDrawWRange [2]float64` — while this mixture is being produced
  - `ThroughputKgHourRange [2]float64` — kg finished propellant per hour
- **`RefineryArchetype`** — template:
  - `Name`, `Description`
  - `TechTier int` — mirrors civ/archetype tier gating
  - `HealthInitRange [2]float64`
  - `DryMassKgRange [2]float64`
  - `IdlePowerDrawWRange [2]float64` — draw when not refining (containment, thermal)
  - `Productions []MixtureProduction` — one entry per supported mixture
- **`Refinery`** instance — embeds `factory.SystemBase`, carries rolled scalars. Not produced in Phase 4 beyond the type definition.
- **Empty registry.** No archetypes registered in Phase 4. The registration pattern, validation hooks, and picker are in place; user authors content later.

### Loadout change — deferred
`ShipLoadout` does *not* gain a `Refinery` field in Phase 4. The subpackage exists, validation runs on an empty registry, and `GenerateRandomShip` stays untouched. Loadout wiring lands when the user has authored at least one refinery archetype. Noted explicitly in §6.

### Scope guardrails
- **No runtime mechanics.** No catalyst wear tick, no heat accumulation, no dirty-fuel state, no breakdown risk, no refining-time simulation.
- **No archetype instances.** Registry is empty.
- **No ship integration.** `ShipLoadout.Refinery` does not exist yet.
- Recipes live on productions, not mixtures. One mixture can have many production paths.

### Validation (empty-registry-safe)
- Every `Production.MixtureID` resolves in `factory.Mixtures`.
- Every `Production.Recipe` entry resolves in the resource registry and has `Category == WildPrecursor`.
- `Production.CatalystID` resolves and has `Category == Catalyst`.
- `RefineryArchetype.Productions` is non-empty *if* the archetype is registered. Empty registry passes trivially.
- Coverage invariant deferred until content exists. Documented as a TODO on the validator.

---

## 4. Time Dilation - Design Doc Only

Far drives force the question of shared time. Phase 4 does not implement any of it — no `GlobalEpoch`, no `ProperTime`, no Lorentz calculation in the physics loop, no decay-routing changes. That's a later phase.

What Phase 4 *does* land: a new engineering design doc `Design Documents/TE_TimeDilation.md`, sibling to `TE_Flight.md`, capturing the intended model for future reference:

- One universal clock (`GlobalEpoch`) shared across the sim.
- Per-ship proper time (`ProperTime`) that diverges from `GlobalEpoch` at high γ.
- The Lorentz math: `ΔtShip = ΔtGlobal / γ`.
- Decay-channel routing — what ticks against ship time (engineering wear, battery drain, catalyst consumption) vs. what ticks against global time (probes, beacons, derelict wrecks, anything detached from the ship).
- Scope notes on what's *not* being solved yet (Far transit session flow, client-side dilation effects, aging UI).

The doc is reference material for future implementers — including future-me. No code lands this phase.

---

## 5. FactoryVersion

Archetype expansion, mixture shape change, new resource registry, and the Refinery addition to `ShipLoadout` are all breaking. Bump `FactoryVersion` when Phase 4 ships. Old ships stay readable (JSONB is permissive) but regeneration against a Phase 4 factory produces the new shape.

No migration of existing rows. `phase3-v2` ships stay that way until regenerated via `/api/ships/generate`.

---

## 6. Out of Scope (Explicit)

- Damage / repair / break-type model.
- **Refinery runtime mechanics** — catalyst wear tick, heat, dirty fuel, breakdown risk, refining-time simulation. Declared, not simulated.
- **Refinery archetype content** — no authored archetypes, no instances, no `ShipLoadout.Refinery` field yet. Subpackage + types + validation only.
- **Resource / mixture / ignition content authoring** — registry types land empty; user fills them in post-infra.
- Refueling UX, gathering loop, inventory on the ship.
- Scanners, probes, Spacecraft OS.
- Signal propagation, universe generation, spatial indexing.
- **Any time-dilation implementation** — `GlobalEpoch`, `ProperTime`, Lorentz factor, decay routing. Design doc only.
- Session flow for Far transit (commit / arrival / wake events).
- Cross-category balance (power budget closure, thermal closure, delta-V targets).
- Client-side work of any kind.

---

## 7. Phase 4 Validation

End-of-phase acceptance:

- Tier-5 ships generate with Short, Medium, and Far slots all non-null.
- Tier-1..4 ships generate with `Far: <null>` and validation passing.
- `server/internal/factory/resources.go` compiles with an empty registry and a `registerResource` helper ready for content.
- `factory.Mixture` has `IgnitionNeed *ResourceID` and `Synthetic bool`; existing mixtures still compile with zero-value fields.
- `server/internal/factory/refinery/` compiles with `RefineryArchetype`, `MixtureProduction`, and an empty registered-archetype list.
- Server startup runs validation over all registries (resources, mixtures, flight archetypes, refinery archetypes) without panicking on emptiness.
- `TE_TimeDilation.md` exists in the design documents tree.
- `FactoryVersion == "phase4-v1"`.

---

## 8. Open Questions

- **Unresolved mixture references on authored archetypes.** `RDEShockwave`, `SABRE`, `TCAShort`, `PDDPhotolytic`, `HPFAService` reference mixtures that don't exist yet (`Methane_Fluorine`, `Polyphosphate_Concentrate`, `HTP_90`, `Hydrazine_Mono`, `Methalox`, `Glass-Hydrazine`, `Aerozine50_NTO`, `Hydrolox`). In Phase 4 the mixture-exists validator is relaxed to *warn* rather than panic so infra can land; user authors the missing mixtures or reroutes references post-infra.
- Whether `IgnitionNeed` stays a single `ResourceID` or becomes a small slice. Current plan: single pointer. Upgrade if any archetype legitimately needs two.
- Whether `MixtureProduction` should carry a quality/purity scalar so multi-level refineries differ in *output quality* (feeding Isp multipliers downstream), not just throughput and power. Deferred to post-content pass.
- Whether refinery archetypes should gate by `TechTier` the same way flight archetypes do. Current plan: yes, `RefineryArchetype.TechTier` is declared, gating logic deferred to content pass.
