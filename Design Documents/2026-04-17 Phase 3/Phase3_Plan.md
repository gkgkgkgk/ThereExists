# Phase 3 Plan — The Factory
*2026-04-17*

## Goal
Stand up the **Factory** — the part of the system responsible for procedural assembly of all spacecraft (and, by the end of the phase, probes). Ship generation grows from "empty loadout" (Phase 2) into a constraint-driven assembly that produces internally-consistent vehicles whose visuals reflect their systems.

The factory lives **in-process inside the Go server** as `internal/factory`. We considered a separate Python service and rejected it: the same calculations (`DeltaV`, `PowerBudget`, `ThermalLoad`, …) are needed both at *factory time* (validating generated configs) and at *runtime* (live ship state, engineering changes), and crossing a process boundary for every one of those calls — or duplicating them in two languages — is a tax that doesn't buy us anything yet. If we later need Python's strengths (LLM-generated names/lore, scipy-grade optimization) we add a narrow sidecar for *just that thing* rather than splitting the factory.

The user-facing payoff is unchanged: every player gets a ship that *makes sense* (its power budget closes, its thrusters match its mass, its radiators match its thermal load) and *looks* like the systems it carries.

---

## 1. Architecture

```
┌────────┐    GET /api/player        ┌────────────────────────┐
│ Client │ ────────────────────────► │   Go API (server/)     │
│  (TS)  │ ◄──────────────────────── │                        │
└────────┘     player + ship         │ ┌────────────────────┐ │
                                     │ │ internal/factory   │ │
                                     │ │  - catalog.yaml    │ │
                                     │ │  - assembler       │ │
                                     │ │  - solver          │ │
                                     │ │  - system models   │ │
                                     │ └────────────────────┘ │
                                     │            │           │
                                     │            ▼           │
                                     │      ┌─────────┐       │
                                     │      │ Postgres│       │
                                     │      └─────────┘       │
                                     └────────────────────────┘
```

- **Client** unchanged in shape — still hits `GET /api/player`, still receives a player object that now embeds a richer ship loadout.
- **Go API** owns persistence (`players`, `ships`), session, and routing.
- **`internal/factory` package** owns the catalog data file, system models (structs + methods), and the assembly logic. Pure Go, no I/O beyond reading the catalog file at startup.

### Why in-process
- **No duplicated math.** The same `(*Thruster).DeltaV(...)` method is called by the assembler when validating a generated ship and by gameplay code when the player burns fuel.
- **One deploy target.** No new container, no new CI job, no service-to-service auth, no schema-mirroring between languages.
- **Testable in isolation.** The factory package has no DB or HTTP dependencies, so its tests are fast and pure.
- **Easy to extract later.** If the factory grows enough to warrant its own service, a Go package with a clear interface is a much easier thing to lift out than tangled handler code.

---

## 2. The Factory Package

### Starting layout — one system category at a time
We start tiny: a single file that knows how to make a **liquid chemical engine**, plus shared scaffolding (`SystemBase`, mixtures registry). Once that shape settles, the same pattern is repeated for other propulsion categories (solid, ion, cold gas, nuclear thermal, …) and eventually for other system categories (power, thermal, sensors, …) as sibling files. A `propulsion.go` with a `PropulsionSystem` interface + dispatcher gets added once we have a second propulsion category to share it with.

```
server/internal/factory/
  system_base.go         // SystemBase: identity fields shared across every system
  mixtures.go            // propellant mixture registry (hand-authored)
  manufacturers.go       // manufacturer registry (hand-authored, HIL)
  propulsion_liquid.go   // liquid chemical: archetype + instance + generator
```

Everything else (`catalog.yaml`, full `assembler.go`, `solver.go`, cross-category constraints) stays deferred.

### The archetype + instance pattern
Every system is generated via **archetype + instance**:

- An **archetype** is a static, code-defined template: ranges for scalar parameters, allowed-lists for enums, and foreign keys (e.g. propellant mixture IDs). Archetypes live as package-level `var`s and don't change at runtime. Multiple archetypes can exist per category (e.g. `RCSLiquidChemical`, `OMSLiquidChemical`, `BoosterLiquidChemical`, `HypergolicStorable`, `CryogenicUpperStage`).
- An **instance** is a concrete system rolled from an archetype with a seeded `*rand.Rand`. Instances are what gets persisted to `ships.loadout` and mutated at runtime (wear, restarts, health, temperature).

Generator signature is always the same shape:

```go
func GenerateLiquidChemicalEngine(arch LiquidChemicalArchetype, rng *rand.Rand, qt QualityTier) *LiquidChemicalEngine
```

The caller owns the `*rand.Rand` so ship-level generation can thread one seed lineage through every subsystem deterministically. `QualityTier` is rolled once at ship level and passed down.

### `SystemBase` — shared across every system
Every system instance embeds `SystemBase`. Defining it now — before a second system type exists — avoids retrofitting identity fields across every system type later. Cost today: ~15 lines.

```go
type SystemBase struct {
    ID             uuid.UUID   // unique per instance
    Name           string      // resolved from manufacturer table + model code
    ArchetypeName  string      // which archetype produced this instance
    ManufacturerID string      // FK into hand-authored manufacturer registry
    SerialNumber  string       // procedural, e.g. "KR-7-A-00142"
    QualityTier   QualityTier  // Prototype / Standard / Refurbished / Salvaged
    Health        float64      // 0.0–1.0, mutable, initialized by QualityTier
}
```

### `LiquidChemicalArchetype` — fields
Ranges are `[2]float64` (or `[2]int`) unless noted. Field order below matches the generation DAG in the next section. No range is sampled uniformly when a dependency exists — dependents are conditioned on prior rolls.

- **Performance driver**: `ChamberPressureRange` (bar). Rolled *first* — drives Isp ceiling, cooling demand, and thrust density. Typical: 5–300 bar.
- **Cooling**: `AllowedCoolingMethods` (filtered at runtime by chamber pressure — e.g. Ablative impractical above ~150 bar, Radiative only below ~40 bar).
- **Performance**: `IspVacuumRange` (s), `IspAtRefPressureRange` (s; only the *low* bound is used — the high bound is `IspVacuumSec` from the prior roll), `ReferencePressurePa` (scalar, typically 101325), `ThrustVacuumRange` (N, log-uniform — spans orders of magnitude).
- **Physical**: `DryMassRange` (kg, log-uniform).
- **Gimbal**: `GimbalEligibleMassKg` (scalar; below this mass, no gimbal roll), `GimbalRangeRange` (deg, only sampled if eligible).
- **Power**: `IgnitionPowerWRange` (W, peak during ignition — zero for hypergolic/pyrotechnic), `OperatingPowerWRange` (W, steady-state — valves, TVC actuators, electronics; electric-pump-fed archetypes run 100–1000× higher).
- **Propellant & ignition**: `AllowedMixtureIDs` (FKs into `mixtures.go`). `IgnitionMethod` is *derived* from the rolled mixture's flags, not declared independently.
- **Operational envelope**: `MaxContinuousBurnRange` (s), `MaxRestarts` (int; `-1` = unlimited, always use `HasRestartsRemaining()` helper), `MinThrottleRange`, `MaxThrottleRange` (typically 1.0–1.2; >1.0 = overthrottle, accelerates wear).
- **Thermal**: `RestartTemperatureCeilingKRange` (K, chamber temperature below which a restart is safe — inversely correlated with chamber pressure).
- **Ablator** (only meaningful if `Ablative` ∈ `AllowedCoolingMethods`): `AblatorMassKgRange` (kg).

### `LiquidChemicalEngine` — fields
Embeds `SystemBase`. Grouped by mutability.

**Immutable after generation:**
- Performance: `ChamberPressureBar`, `IspVacuumSec`, `IspAtRefPressureSec`, `ReferencePressurePa`, `ThrustVacuumN`
- Physical: `DryMassKg`
- Configuration: `PropellantConfig`, `IgnitionMethod`, `CoolingMethod`, `MixtureID`
- Power: `IgnitionPowerW`, `OperatingPowerW`
- Operational envelope: `MinThrottle`, `MaxThrottle`, `CanThrottle`, `MaxContinuousBurnSeconds`, `MaxRestarts`, `GimbalRangeDegrees`, `CanGimbal`
- Thermal: `RestartTemperatureCeilingK`
- Ablator: `InitialAblatorMassKg` (0 if non-ablative)

**Mutable runtime state (persisted):**
- `RestartsUsed` (int), `TotalBurnTimeSeconds`, `CurrentBurnSeconds` (since last ignition), `IsFiring` (bool)
- `CurrentTemperatureK` — initialized to ambient at generation; integrated per-tick in later phases
- `AblatorMassRemainingKg` — starts at `InitialAblatorMassKg`, depletes per burn-second (Ablative only)

### Generation order — dependency-grouped
The generator is a DAG, not a flat uniform-sample pass. Each group depends on prior groups' outputs; no clamps or post-hoc fixups are needed because dependent samples are drawn from ranges *already bounded* by prior rolls.

**Group 0 — `SystemBase` (cross-cutting)**
- `ID` (`uuid.UUID`): fresh UUIDv4.
- `ManufacturerID` (`string`): rolled from ship-archetype's manufacturer weights (hand-authored — see Manufacturers below).
- `Name` (`string`): resolved from `{manufacturer.NamingConvention}(rng, archetype)`.
- `SerialNumber` (`string`): procedural `{manufacturer}-{archetype_code}-{rng_suffix}`.
- `ArchetypeName` (`string`): copied from archetype.
- `QualityTier` (`QualityTier`): passed in from ship-level generation.
- `Health` (`float64`): `1.0` for Prototype/Standard, `0.7–0.95` for Refurbished, `0.4–0.8` for Salvaged.

**Group 1 — Performance driver (no dependencies)**
- `ChamberPressureBar` (`float64`, bar): uniform-sample `ChamberPressureRange`. Everything downstream is conditioned on this.

**Group 2 — Cooling (depends on Group 1)**
- `CoolingMethod` (`CoolingMethod` enum): filter `AllowedCoolingMethods` by chamber pressure band, pick uniformly from survivors.
- `InitialAblatorMassKg` (`float64`, kg): if `CoolingMethod == Ablative`, sample `AblatorMassKgRange`; else `0`.
- `AblatorMassRemainingKg`: initialized to `InitialAblatorMassKg`.

**Group 3 — Performance (depends on Groups 1 & 2)**
- `IspVacuumSec` (`float64`, s): sample `IspVacuumRange`, distribution biased toward upper end proportional to `ChamberPressureBar`.
- `IspAtRefPressureSec` (`float64`, s): sample from `[IspAtRefPressureRange.lo, IspVacuumSec]` — by construction cannot exceed `IspVacuumSec`. No clamp needed.
- `ReferencePressurePa` (`float64`, Pa): copied from archetype.
- `ThrustVacuumN` (`float64`, N): log-uniform sample across `ThrustVacuumRange`, biased upward by `ChamberPressureBar`.

**Group 4 — Physical (depends on Group 3)**
- `DryMassKg` (`float64`, kg): log-uniform sample across `DryMassRange`, biased upward by `ThrustVacuumN` (bigger thrust → bigger engine).

**Group 5 — Gimbal (depends on Group 4)**
- If `DryMassKg < GimbalEligibleMassKg`: `CanGimbal = false`, `GimbalRangeDegrees = 0`.
- Else: `CanGimbal = true`, sample `GimbalRangeRange`.

**Group 6 — Power (depends on Group 1)**
- `IgnitionPowerW` (`float64`, W): sample `IgnitionPowerWRange`. Zero for pyrotechnic/hypergolic archetypes.
- `OperatingPowerW` (`float64`, W): sample `OperatingPowerWRange`.

**Group 7 — Propellant & ignition**
- `MixtureID` (`string`): pick uniformly from `AllowedMixtureIDs`.
- `PropellantConfig` (`PropellantConfig` enum): derived from mixture's `Config` flag.
- `IgnitionMethod` (`IgnitionMethod` enum): derived from mixture flags — hypergolic mixture → `Hypergolic`; monopropellant → `Catalytic`; else pick uniformly from `{Spark, Pyrotechnic}`.

**Group 8 — Operational envelope (depends on Groups 1, 2, 4, 7)**
- `MaxContinuousBurnSeconds` (`float64`, s): sample, penalized by Ablative cooling.
- `MaxRestarts` (`int`): copied from archetype (`-1` = unlimited). Cryogenic mixtures typically use archetypes with lower caps.
- `MinThrottle`, `MaxThrottle` (`float64`): sample. `CanThrottle = MaxThrottle > MinThrottle`; if false, clamp both to `1.0`.
- `RestartTemperatureCeilingK` (`float64`, K): sample `RestartTemperatureCeilingKRange`, correlated inversely with `ChamberPressureBar`.

**Group 9 — Runtime state initialization**
- `RestartsUsed = 0`, `TotalBurnTimeSeconds = 0`, `CurrentBurnSeconds = 0`, `IsFiring = false`.
- `CurrentTemperatureK = 288.15` (15°C default; ship-level generation can override for environmental context).

### Enums
```go
type PropellantConfig int   // Monopropellant, Bipropellant
type IgnitionMethod  int    // Spark, Pyrotechnic, Hypergolic, Catalytic
type CoolingMethod   int    // Ablative, Regenerative, Radiative, Film
type QualityTier     int    // Prototype, Standard, Refurbished, Salvaged
```

Each has `String()` for logging/display.

### Mixtures (hand-authored registry)
`mixtures.go` holds a small pre-defined table. Each entry:

```go
type Mixture struct {
    ID              string            // "LOX_LH2", "MMH_NTO", "Hydrazine"
    Config          PropellantConfig  // Mono or Bipropellant
    IspMultiplier   float64           // applied to engine's base Isp
    DensityKgM3     float64           // bulk density, for tank sizing later
    StorabilityDays int               // -1 = indefinite
    Hypergolic      bool              // ignites on contact → forces IgnitionMethod = Hypergolic
    Cryogenic       bool              // requires active cooling; typically caps restarts
}
```

Mixtures are pre-defined (not generated) because they propagate cross-system: an engine's `MixtureID` must match a tank the ship will later carry. Keeping the registry small and hand-authored makes that cross-category constraint trivial.

### Manufacturers (hand-authored, human-in-the-loop)
Manufacturer names, house styles, and naming conventions are **not procedurally generated at runtime**. `manufacturers.go` holds ~20–30 hand-authored entries:

```go
type Manufacturer struct {
    ID                string
    DisplayName       string
    NamingConvention  func(rng *rand.Rand, arch string) string  // produces model name
    ArchetypeWeights  map[string]float64                         // biases which archetypes pull this manufacturer
    Flavor            string                                     // UI blurb
}
```

Authoring this table is a separate HIL pass — procedural-only naming produces slop, and manufacturer flavor is load-bearing for the game's identity.

### Cooling methods — each has real teeth
No `ThermalLoadFactor` field. A future `HeatToHullW(throttle) float64` method (spec'd now, not implemented in Phase 3) computes heat leak into the ship's hull from `(throttle, ChamberPressureBar, CoolingMethod, Health, AblatorMassRemainingKg)`:

- **Regenerative**: near-zero `HeatToHullW`, no consumables; fails catastrophically above a pressure/throttle envelope (sharp Health loss).
- **Ablative**: near-zero `HeatToHullW`, but `AblatorMassRemainingKg` depletes per burn-second scaled by throttle. Engine bricks when it hits zero.
- **Radiative**: low steady-state `HeatToHullW`, but forces a lower `MaxThrottle` ceiling (can't dump fast enough).
- **Film**: low `HeatToHullW`, but effective `IspAt(ambientPa)` is reduced by a film-coolant penalty factor.

`HeatToHullW` is consumed by the future **ship-level thermal bus** (radiators + coolant loops), which aggregates heat leak from all subsystems and handles ship-wide rejection. The thermal bus is not in Phase 3 scope; the engine just exposes the getter.

### Isp interpolation
```go
func (e *LiquidChemicalEngine) IspAt(ambientPa float64) float64 {
    t := ambientPa / e.ReferencePressurePa
    isp := e.IspVacuumSec + t*(e.IspAtRefPressureSec - e.IspVacuumSec)
    if isp < 0 { isp = 0 }  // nozzle overexpansion collapse
    return isp
}
```

No upper clamp on `t`: linear extrapolation drives Isp to zero in dense atmospheres, correctly modeling flow separation on a vacuum-optimized nozzle.

### Wear model (spec'd, not implemented in Phase 3)
A future `Tick(dt, throttle)` method will mutate `Health`, `TotalBurnTimeSeconds`, `CurrentBurnSeconds`, `CurrentTemperatureK`, and `AblatorMassRemainingKg`. Wear rate shape:

```
wear_rate = base_wear * throttle_factor(throttle) * cooling_factor(CoolingMethod) * (2.0 - Health)
throttle_factor(x) = 1.0                       if x <= 1.0
throttle_factor(x) = 1.0 + k * (x - 1.0)^3     if x > 1.0    // superlinear above nominal
```

The `(2.0 - Health)` term creates positive feedback — damaged engines wear faster. Constants `k`, `base_wear`, and the per-cooling-method coefficients are TBD when the simulation phase starts.

Restart-readiness is a separate check: `CurrentTemperatureK < RestartTemperatureCeilingK`. Environmental ambient affects cooldown time.

### Archetype validation
A `func (a LiquidChemicalArchetype) Validate() error` runs at package `init()` and panics on internally-contradictory archetypes — e.g. `GimbalEligibleMassKg < DryMassRange.lo` (every engine gimbals, gating is dead code), or `AllowedMixtureIDs` containing only hypergolic mixtures while `AllowedCoolingMethods` is empty after pressure filtering for any realistic chamber pressure. Most single-field range invariants (like `IspVacuumSec ≥ IspAtRefPressureSec`) are structurally guaranteed by the DAG generation order and do not need to be re-checked.

### Example archetype (v1)
```go
var RCSLiquidChemical = LiquidChemicalArchetype{
    Name:                             "RCSLiquidChemical",
    ChamberPressureRange:             [2]float64{5, 25},
    IspVacuumRange:                   [2]float64{220, 290},
    IspAtRefPressureRange:            [2]float64{180, 240},
    ReferencePressurePa:              101325,
    ThrustVacuumRange:                [2]float64{50, 1_000},        // log-uniform
    DryMassRange:                     [2]float64{1, 50},            // log-uniform
    GimbalEligibleMassKg:             9999,                         // RCS never gimbals
    GimbalRangeRange:                 [2]float64{0, 0},
    IgnitionPowerWRange:              [2]float64{0, 20},
    OperatingPowerWRange:             [2]float64{5, 50},
    AllowedMixtureIDs:                []string{"MMH_NTO", "Hydrazine"},
    AllowedCoolingMethods:            []CoolingMethod{Ablative, Radiative, Film},
    MaxContinuousBurnRange:           [2]float64{1, 300},
    MaxRestarts:                      -1,                           // unlimited
    MinThrottleRange:                 [2]float64{1.0, 1.0},         // on/off
    MaxThrottleRange:                 [2]float64{1.0, 1.0},
    RestartTemperatureCeilingKRange:  [2]float64{400, 600},
    AblatorMassKgRange:               [2]float64{0.1, 2.0},
}
```

Additional archetypes sketched for later: `OMSLiquidChemical`, `SustainerLiquidChemical`, `BoosterLiquidChemical`, `HypergolicStorable`, `CryogenicUpperStage`. V1 requires only one to be end-to-end working.

### Archetype authoring cost
A single range-based archetype produces wide variety — one archetype can cover a real thrust-tier's worth of engines. If a given archetype balloons past ~200 lines or authoring a new one feels painful, escalate:
1. `DefaultLiquidChemical` with sensible ranges, per-archetype sparse overrides.
2. Derive rather than declare where possible.
3. Builder/functional-options for authoring.

Not a blocker for v1. Flag the concern if authoring the third archetype feels painful.

### Dispatcher (stub today, real later)
`GenerateRandomEngine(rng, qt) (PropulsionSystem, error)` maintains a registry of all archetypes across all propulsion categories. Selection policy: pick *category* uniformly (or with rarity weights — exotic drives rarer), then pick *archetype within category*. Category-first ordering means adding a new category doesn't dilute the visibility of existing categories.

Today the registry holds one archetype. The `PropulsionSystem` interface is deferred until the second category lands.

### Public API (kept narrow)
```go
factory.GenerateLiquidChemicalEngine(arch, rng, qt)   // today
factory.GenerateRandomEngine(rng, qt)                 // today — stub dispatcher
// later: GenerateRandomShip(seed) composes engine + power + thermal + ...
```

Handlers only ever call `GenerateRandomShip` once it exists. Until then, `ships.loadout` contains only a propulsion entry.

### New endpoints
| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/ships/regenerate` | Admin/debug — re-roll the active ship for a player |

`GET /api/player` shape is unchanged; `ship.loadout` becomes non-empty.

---

## 3. Catalog Data *(deferred)*

The original plan called for a YAML catalog embedded into the binary. We're deferring that until all system types exist as code. The shape below is recorded so future-us has a clear migration target if/when the variant count outgrows hand-written structs.

```yaml
version: 2026-04-17
systems:
  - id: flight.ion.mk2
    type: flight
    name: Ion Thruster Mk II
    tags: [efficient, low-thrust]
    attributes:
      thrust_n: 0.25
      isp_s: 4200
      power_draw_w: 2400
      mass_kg: 180
      thermal_load_w: 600
    visual:
      thruster_geom: ion_quad
      nozzle_count: 4
    compatible_with:
      power: ["rtg.*", "fission.*"]
```

### Types (unchanged from Phase 2)
`flight`, `power`, `sensor`, `structural`, `thermal`, `life_support`, `propellant`, `engineering`, `probe`

### What's new vs Phase 2
- `tags` — for archetype scoring (scouts prefer `low-mass`, `efficient`).
- `attributes` — typed numeric fields used by the solver and by runtime calculations.
- `compatible_with` — soft compatibility hints (type → glob patterns over `id`) used to prune the search space.
- `visual` — geometry hints the frontend uses to assemble the mesh.

### Ownership
The catalog is a **data file in the Go repo**, embedded into the binary at build time. No DB table, no migration story, no separate-service deploy. Editing the catalog is a code change, reviewed in PRs like everything else.

---

## 4. Ship Generation Algorithm

### Now (per category)
Each `Generate<Category>(archetype, rng)` samples scalar ranges uniformly, resolves cross-field constraints (e.g. ignition ↔ propellant compatibility in liquid chemical), initializes runtime state, and returns a concrete instance. Caller owns the `*rand.Rand` so a single player seed can be threaded deterministically through every subsystem.

The **dispatcher** (`GenerateRandomEngine`) picks an archetype from the category-wide registry and forwards to the matching generator. Today the registry holds one entry.

### Later (`GenerateRandomShip`)
Once the other system categories exist, generation is a **linear pipeline**, not a backtracking solver. Engines are rolled first; their combined dry mass + thrust class sets the ship's size envelope, and every subsequent category rolls within that running budget.

1. Seed the RNG from the player seed.
2. Roll `QualityTier` at ship level (passed to every subsystem generator).
3. Pick a **ship archetype** (scout / freighter / explorer) that biases which per-category archetypes are pulled from their registries.
4. **Generate engine(s) first.** Their `DryMassKg` + `ThrustVacuumN` define the ship's mass class.
5. Generate propellant tanks conditioned on engine(s)' `MixtureID`.
6. Generate power, thermal bus, sensors, structural, etc. — each category receives the running mass/power/thermal budget from prior groups and rolls within it.
7. Compute derived stats via methods on the instances (Δv, power balance, peak thermal load, `HeatToHullW`).
8. Compute visual block from chosen variants.
9. Return the assembled `ShipConfig`.

No backtracking: engines-first establishes the envelope, and each later category is sized to fit. If a category's generators can't find a valid instance within the remaining budget, that's an archetype-authoring bug caught by `Validate()`, not a runtime concern.

---

## 5. Go API Changes

### New responsibilities
- Construct a `factory.Factory` at startup, pass into handlers alongside `*sql.DB`.
- On player creation (Phase 2 already inserts an empty ship): replace the empty `INSERT ships` with a factory call, persist the resulting `loadout`, `derived`, `visual`, and snapshot the `factory_version`.
- On `POST /api/ships/regenerate`: same flow but for an existing player; current active ship gets `status='derelict'`, new one gets created, `players.active_ship_id` is repointed.

### `GET /api/player` response shape
Already returns `ship`. In Phase 3 the embedded `ship.loadout` becomes meaningful:

```json
{
  "id": "...",
  "seed": 1234567890,
  "ship": {
    "id": "...",
    "loadout": {
      "archetype": "scout",
      "factory_version": "2026-04-17",
      "systems": [
        { "id": "...", "type": "flight",  "name": "Ion Thruster Mk II", "attributes": {...} },
        { "id": "...", "type": "power",   "name": "RTG-7",              "attributes": {...} }
      ],
      "derived": {
        "dry_mass_kg": 38420,
        "delta_v_ms": 9100,
        "peak_power_w": 12400,
        "thermal_load_w": 8800
      },
      "visual": {
        "hull_form": "needle",
        "thruster_geom": "ion_quad",
        "panel_area_m2": 22.4,
        "radiator_area_m2": 9.0,
        "accent_hue": 217
      }
    },
    "state": {},
    "transform": {...},
    "status": "active",
    "created_at": "..."
  }
}
```

`loadout` is set once at factory time and rarely mutates. `state` (Phase 2) is the live mutable side — fuel, sensor cache, etc. — and stays empty until later phases populate it.

---

## 6. DB Changes

The Phase 2 `ships` table (`id`, `player_id`, `loadout`, `state`, `transform`, `status`, `created_at`) already covers what we need. Phase 3 adds nothing structural — `loadout` just stops being `'{}'`.

Two small additions worth making:

| Change | Notes |
|---|---|
| `ships.archetype TEXT NULL` | Hot-path lookups ("show me all freighters in this sector") shouldn't have to crack open `loadout` JSONB. Cheap denormalization. |
| `ships.factory_version TEXT NULL` | Records which catalog version produced the ship. Lets us reason about ships generated against old catalogs after updates. |

Snapshotting the full system objects inside `loadout` (rather than referencing catalog IDs by reference) means a catalog change never silently mutates an existing player's ship. The catalog is a generator input, not a runtime lookup.

---

## 7. Frontend Changes

The frontend **reads** the new ship loadout but most of the visual changes are scaffolding for later phases. Specifically in Phase 3:

- Replace the hardcoded `buildSpacecraft(rng)` in `client/src/scene.js` with a `buildSpacecraft(shipVisual)` that consumes the `visual` block from the API response.
- Hull form, thruster geometry, panel area, radiator area, and accent hue all become data-driven.
- Phase 2's placeholder geometry is retained as the *fallback* when fields are missing.
- HUD gets a small "SHIP: <archetype>" line so the player can see which archetype they rolled.

No cockpit changes in this phase — that lives in Phase 4 (system-specific cockpit instruments).

---

## 8. Testing

- **`factory_test.go`** — pure unit tests on the assembler. No DB, no HTTP. Fast.
  - Determinism: same seed → same ship.
  - Constraint satisfaction: every generated ship's derived stats must satisfy the archetype's constraints.
  - All catalog systems are reachable: fuzz over many seeds, assert every catalog entry shows up in *some* generated ship across N rolls.
- **Handler tests** — table-driven, hit a real Postgres in CI (existing pattern).

---

## 9. Open Questions

- **Archetype coverage for v1**: one liquid chemical archetype is the minimum bar. How many more before moving laterally to other system categories? *Lean: ship one archetype end-to-end, then go laterally (power/thermal/sensors) rather than pile up propulsion variants.*
- **Manufacturer table authoring**: a separate HIL pass with the user — not in scope for the first factory PR, but blocks end-to-end naming.
- **Wear-model constants** (`k`, `base_wear`, per-cooling-method coefficients): TBD until the simulation phase starts. Shape is locked; numbers are not.
- **Dispatcher rarity weights**: uniform for v1; when do we introduce rarity tiers for exotic drives? Punt until there are enough categories to make "rare" meaningful.
- **Probe factory in scope?** Listed below as a stretch goal. Same package, new entry point.

---

## 10. Out of Scope for Phase 3

- Flight controls / Δv simulation (player flying the ship)
- Sensor scanning gameplay
- Spacecraft OS CLI
- Multi-player traces / shared universe state
- AI co-pilot
- Economy / resource consumption
- Cockpit instruments tied to specific systems (Phase 4)

---

## Phase 3 Definition of Done

- A new player loads the app, hits `GET /api/player`, and the Go server's factory package generates a ship inline before responding.
- The returned ship has internally-consistent derived stats: power closes, mass under cap, thermal load served.
- The frontend renders a placeholder ship whose **shape** changes visibly with the rolled archetype and systems (different thruster, different panel size).
- The factory has at least 6 catalog entries per system type and 3 archetypes.
- `factory_test.go` covers determinism and constraint satisfaction.
- `POST /api/ships/regenerate` works end-to-end and correctly transitions the previous ship to `derelict`.
