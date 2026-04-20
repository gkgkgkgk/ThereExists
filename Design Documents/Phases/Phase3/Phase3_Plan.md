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

### Starting layout — per-category subdirectories
Scaffolding lives at the package root; each system category gets its own subdirectory. This scales cleanly as new categories are added without file-name prefix bingo.

```
server/internal/factory/
  system.go              // SystemBase — identity fields shared across every system
  civilizations.go       // Civilization registry (hand-authored) — TechTier + TechProfile live here
  manufacturers.go       // Manufacturer registry (hand-authored, HIL) — each has a CivilizationID
  mixtures.go            // propellant mixture registry (hand-authored)
  flight/
    flight.go            // FlightSlot enum + slot-fill logic (shared FlightSystem interface lands with 2nd flight category)
    liquid.go            // liquid chemical: archetype + instance + generator
    // future: ion.go, solid.go, cold_gas.go, fusion.go, nls_*.go, ...
  // future sibling dirs: power/, thermal/, sensors/, structural/, life_support/, ...
```

Everything else (`assembler.go`, `solver.go`, cross-category constraints) stays deferred.

### Flight slots — every ship has three engines, all mandatory
Every finished ship has exactly one engine per slot. **None of the three slots are optional.** The game's premise is "use what you have to solve problems," not "invent what you're missing" — a player without a Far drive has no path off their starting system and nothing to do, so Far is as mandatory as Short.

- **Short** — close-range maneuvering: RCS, docking, sample collection, mining. Seconds-to-minutes burn, small thrust, high restart count.
- **Medium** — orbital maneuvering, planetary takeoff/landing, intra-system transit. Minutes-to-hours burn, order(s) of magnitude more thrust.
- **Far** — interstellar transit. Near-light-speed drives (NLS). Deliberately soft sci-fi: players can't wait real-world decades to reach other star systems. Future archetypes will lean into speculative physics (Alcubierre-style bubbles, antimatter torches, fusion ramjets, laser-pushed lightsails, etc.). **Far drives vary enormously in capability** — a `TopSpeedFractionC` (or equivalent) range on each Far archetype lets one archetype peak at 0.95c while another peaks at 0.001c. That spread *is* the primary gameplay dial at interstellar scale: some players are a Tuesday from their neighbor's star; others are decades away even at full burn.

Every flight archetype declares which `FlightSlot` it fills. One archetype → one slot. `RCSLiquidChemical` → Short; future `OMSLiquidChemical` / `BoosterLiquidChemical` → Medium; future NLS archetypes → Far.

**Provenance-consistency.** When a ship rolls all three engines, the generator biases toward manufacturers from the same civilization across slots — a ship's engines should feel like one program, not a yard sale. Ships whose three engines span multiple civilizations read as "salvaged" without needing a `Salvaged` label; the effect emerges from the provenance chain.

**Phase 3 scope.** The "all three mandatory" invariant is a *finished-game* constraint. Phase 3 is a development milestone: only the Short slot has an archetype (`RCSLiquidChemical`), so generated ships intentionally have `Medium = null` and `Far = null`. These are dev-only incomplete ships, never player-facing. The `FlightSlot` enum + slot-fill logic land now because `GenerateRandomShip` needs them even in the one-slot case, and the stub makes adding Medium/Far a one-line registry entry later.

### The archetype + instance pattern
Every system is generated via **archetype + instance**:

- An **archetype** is a static, code-defined template: ranges for scalar parameters, allowed-lists for enums, and foreign keys (e.g. propellant mixture IDs). Archetypes live as package-level `var`s and don't change at runtime. Multiple archetypes can exist per category (e.g. `RCSLiquidChemical`, `OMSLiquidChemical`, `BoosterLiquidChemical`, `HypergolicStorable`, `CryogenicUpperStage`).
- An **instance** is a concrete system rolled from an archetype with a seeded `*rand.Rand`. Instances are what gets persisted to `ships.loadout` and mutated at runtime (wear, restarts, health, temperature).

Generator signature is always the same shape:

```go
func GenerateLiquidChemicalEngine(arch LiquidChemicalArchetype, rng *rand.Rand) *LiquidChemicalEngine
```

The caller owns the `*rand.Rand` so ship-level generation can thread one seed lineage through every subsystem deterministically. Ship-level `QualityTier` (see §3) influences generation by biasing *which archetype* is picked and *what ranges* the ship-level roller passes down — not via a parameter on this function.

### `SystemBase` — shared across every system
Every system instance embeds `SystemBase`. Defining it now — before a second system type exists — avoids retrofitting identity fields across every system type later. Cost today: ~12 lines.

```go
type SystemBase struct {
    ID             uuid.UUID   // unique per instance
    Name           string      // resolved from manufacturer table + model code
    ArchetypeName  string      // which archetype produced this instance
    ManufacturerID string      // FK into hand-authored manufacturer registry
    SerialNumber  string       // procedural, e.g. "KR-7-A-00142"
    Health         float64     // 0.0–1.0, mutable
}
```

There's no `QualityTier` field — quality is *inherited* through `ManufacturerID → CivilizationID → TechTier` (see §3). That chain is what determines how aggressively an engine's `HealthInitRange` gets narrowed, how exotic a cooling method it's allowed to use, and how tight its throttle envelope can be. The player infers the civilization (and therefore the build quality) from symptoms and from the manufacturer itself — nothing labeled "QualityTier" is ever displayed or persisted on the system.

### `LiquidChemicalArchetype` — fields
Ranges are `[2]float64` (or `[2]int`) unless noted. Field order below matches the generation DAG in the next section. No range is sampled uniformly when a dependency exists — dependents are conditioned on prior rolls.

- **Slot**: `FlightSlot` (enum: Short | Medium | Far) — declared once per archetype, copied onto the generated instance. Governs which ship-slot this archetype is eligible to fill.
- **Health**: `HealthInitRange` ([2]float64, 0.0–1.0) — range the ship-level generator narrows further based on `QualityTier` before the engine generator samples it.
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

**Group 0 — `SystemBase` + slot (cross-cutting)**
- `ID` (`uuid.UUID`): fresh UUIDv4.
- `ManufacturerID` (`string`): rolled from the ship-level civilization-mix policy (see §3). Provenance-consistent ships draw manufacturers from a single civilization's roster; "salvaged" ships roll manufacturers independently per subsystem, often crossing civilizations.
- `Name` (`string`): resolved from `{manufacturer.NamingConvention}(rng, archetype)`.
- `SerialNumber` (`string`): procedural, `{manufacturer}-{archetype_code}-{rng_suffix}`.
- `ArchetypeName` (`string`): copied from archetype.
- `FlightSlot` (`FlightSlot` enum): copied from archetype onto the instance.
- `Health` (`float64`): sampled from `HealthInitRange` *narrowed by the manufacturer's civilization's `TechTier`* — higher-tier civilizations produce engines nearer the top of the range; lower-tier produce nearer the bottom.

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
type FlightSlot      int    // Short, Medium, Far
```

`QualityTier` is defined alongside ship-level generation (§3), not here — it's never stored on a system.

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

### Civilizations (hand-authored)
Every manufacturer belongs to a civilization. A civilization captures two orthogonal axes:

```go
type Civilization struct {
    ID           string
    DisplayName  string
    TechTier     int          // advancement level: N-point scale (e.g. 1–5). Drives Health range narrowing,
                              // throttle-envelope tightness, access to exotic cooling methods, etc.
    TechProfile  TechProfile  // cultural/scientific preferences — orthogonal to TechTier
    Flavor       string       // short UI blurb
}

type TechProfile struct {
    PreferredCoolingMethods []CoolingMethod   // ranking — earlier entries more likely
    PreferredIgnitionTypes  []IgnitionMethod
    PreferredMixtureIDs     []string
    AversionToCryogenics    float64           // 0.0 (fine) – 1.0 (never)
    FarDriveFamily          string            // which NLS archetype family this civilization tends to produce
    // ...grow this struct as flavor axes reveal themselves
}
```

**Why the two axes are separate.** `TechTier` is "how advanced is this civilization's engineering" — a ladder. `TechProfile` is "what does this civilization *prefer* to build" — horizontal flavor. Two Tier-3 civilizations can produce radically different engines: one favors hypergolic storable bipropellants with ablative cooling (reliability-first, easy logistics), the other favors cryogenic bipropellants with regenerative cooling (performance-first, fragile logistics). Fusing these into one ladder would flatten every civilization onto a "good/bad" spectrum and erase the texture.

**Phase 4 — LLM-generated civilizations.** Civilizations are generated by an LLM given a seed prompt: the model emits a short description (species, culture, history, environmental constraints — e.g. "aquatic species on a high-gravity world with abundant rare earths") **and** the structured `TechTier` + `TechProfile` fields in one structured-output call. Keeping description and mechanical fields in a single schema prevents desync — the "loves hypergolic propellants because their homeworld is volcanic and cryogenic storage is impractical" description *has to* match a `TechProfile` that prefers hypergolic ignition and aversion-to-cryogenics.

**Phase 3 — one generic civilization.** A single hand-authored `GenericCivilization` (mid-tier, no strong `TechProfile` biases) sits in `civilizations.go`. Every manufacturer's `CivilizationID` points to it. The provenance chain is exercised end-to-end — so swapping in real LLM-generated civs in Phase 4 is purely a content change, not a schema change.

### Manufacturers (hand-authored, human-in-the-loop)
Manufacturer names, house styles, and naming conventions are **not procedurally generated at runtime**. `manufacturers.go` holds ~20–30 hand-authored entries, each tagged with its parent civilization:

```go
type Manufacturer struct {
    ID                string
    CivilizationID    string                                         // FK into civilizations.go
    DisplayName       string
    NamingConvention  func(rng *rand.Rand, arch string) string       // produces model name
    ArchetypeWeights  map[string]float64                             // biases which archetypes pull this manufacturer
    Flavor            string                                         // UI blurb
}
```

A manufacturer inherits its parent civilization's `TechTier` and `TechProfile` — a Tier-3 civilization's manufacturers all produce Tier-3-grade engines, biased toward that civilization's preferences. This is what makes "who built this ship" mean something in the generator, not just in flavor text.

Authoring both tables is a separate HIL pass — procedural-only naming produces slop, and manufacturer + civilization flavor is load-bearing for the game's identity.

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
    FlightSlot:                       Short,
    HealthInitRange:                  [2]float64{0.85, 1.0},
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
`flight.GenerateForSlot(slot FlightSlot, rng *rand.Rand) (FlightSystem, error)` maintains a registry of all flight archetypes keyed by `FlightSlot`. Selection policy within a slot: pick *category* uniformly (or with rarity weights — exotic drives rarer), then pick *archetype within category*. Category-first ordering means adding a new flight category doesn't dilute the visibility of existing ones.

Today the registry holds one archetype (`RCSLiquidChemical`, slot=Short). Calls for `Medium` or `Far` return a typed "slot-empty" result for now; ship loadout simply omits those slots. The `FlightSystem` interface lands with the second flight category.

### Public API (kept narrow)
```go
flight.GenerateLiquidChemicalEngine(arch, rng)   // today
flight.GenerateForSlot(slot, rng)                // today — stub dispatcher
// later: factory.GenerateRandomShip(seed) composes flight slots + power + thermal + ...
```

Handlers only ever call `GenerateRandomShip` once it exists. Until then, `ships.loadout` contains only a Short-slot flight entry.

### New endpoints
| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/ships/generate` | Manual-only — hit it to roll a ship and inspect the result. Not wired into the UI in Phase 3. |

`GET /api/player` shape is unchanged; `ship.loadout` becomes non-empty once `/generate` has been hit.

---

## 3. Ship Generation Algorithm

### Now (per category)
Each `Generate<Category>(archetype, rng)` runs the dependency DAG from §2, returning a concrete instance. Caller owns the `*rand.Rand` so a single player seed can be threaded deterministically through every subsystem.

The **dispatcher** (`flight.GenerateForSlot`) picks an archetype from the slot-keyed registry and forwards to the matching category generator. Today the registry holds one archetype (`RCSLiquidChemical`, Short slot).

### Civilization-driven generation (the quality knob)
There's no `QualityTier` enum. Build quality and tech flavor are *inherited* from the civilizations that built each subsystem, via the `Manufacturer → Civilization` chain (see §2). Ship-level generation exposes two levers that shape how this plays out:

1. **Primary-civilization roll.** Every ship picks a *primary civilization* at generation time. Each subsystem's `ManufacturerID` roll is then weighted toward manufacturers belonging to that civilization. The result: all three engines feel like they came from the same culture — same naming conventions, same cooling preferences, same tech tier.
2. **Civilization-mix policy.** A small probability is reserved for "salvaged" ships whose subsystems roll manufacturers *independently* (no primary-civilization bias), producing multi-civilization loadouts. This is what gives ships a yard-sale feel without needing a `Salvaged` label — the effect falls out of the provenance chain naturally.

Once a manufacturer is picked, its civilization's `TechTier` narrows that subsystem's `HealthInitRange` (higher tier → nearer the top) and gates exotic options (low-tier civilizations can't produce Regenerative-cooled high-chamber-pressure engines; high-tier civilizations get access to the whole tree). Its `TechProfile` biases cooling/ignition/mixture rolls toward its preferences.

None of this is stored on the ship or any subsystem beyond `ManufacturerID`. Tier is always *read through* the civilization FK. The player learns a civilization's reputation by observing its output — a ship stamped with a Tier-1 primitive manufacturer's logo will clearly be rougher than one from a Tier-5 post-scarcity manufacturer, without any in-game label needing to say so.

**Phase 3 simplification.** With only `GenericCivilization` in the registry, the primary-civilization roll is trivial (always picks the one civ) and the civilization-mix policy has nothing to mix across. The mechanism is wired end-to-end; the variety lands in Phase 4 once LLM-generated civilizations populate the registry.

**Phase 4 hook — player-origin civilization.** Once the LLM-generation pipeline exists (see §2 Civilizations), the *player's civilization of origin* is rolled (or story-generated) and used as the default primary civilization for their starting ship. This is how vague mission framing ("you're a scout from Civilization X, go find *something*") ties into the factory without requiring mission logic now.

### Later (`GenerateRandomShip`)
Once the other system categories exist, generation is a **linear pipeline**, not a backtracking solver. Engines are rolled first; their combined dry mass + thrust class sets the ship's size envelope, and every subsequent category rolls within that running budget.

1. Seed the RNG from the player seed.
2. Roll the **primary civilization** (biased later by player-origin; uniform for now) and the civilization-mix policy (consistent vs. salvaged).
3. Pick a **ship archetype** (scout / freighter / explorer) that biases which per-category archetypes are pulled from their registries.
4. **Generate all three flight slots** (`Short`, `Medium`, `Far`). Combined `DryMassKg` + `ThrustVacuumN` define the ship's mass class.
5. Generate propellant tanks conditioned on each slot's `MixtureID`.
6. Generate power, thermal bus, sensors, structural, etc. — each category receives the running mass/power/thermal budget from prior groups and rolls within it, with civilization-weighted manufacturer rolls threaded throughout.
7. Compute derived stats via methods on the instances (Δv per slot, power balance, peak thermal load, `HeatToHullW`).
8. Compute visual block from chosen variants.
9. Return the assembled `ShipConfig`.

No backtracking: engines-first establishes the envelope, and each later category is sized to fit. If a category's generators can't find a valid instance within the remaining budget, that's an archetype-authoring bug caught by `Validate()`, not a runtime concern.

---

## 4. Go API Changes

### New responsibilities
- Construct the factory package's registries at startup (archetypes, mixtures, manufacturers).
- Implement `POST /api/ships/generate`: roll a ship via the factory, persist the resulting `loadout` into the caller's active `ships` row, and snapshot `factory_version` (code SHA or build tag). This is a **manual-only, debug-style** endpoint in Phase 3 — not wired into the UI and not triggered automatically on player creation.

Player creation still inserts an empty ship (Phase 2 behavior, unchanged). The factory runs only when `/generate` is explicitly hit.

### `GET /api/player` response shape
Already returns `ship`. In Phase 3, after `/generate` has been called for that player, the embedded `ship.loadout` becomes meaningful — but only the Short flight slot is populated:

```json
{
  "id": "...",
  "seed": 1234567890,
  "ship": {
    "id": "...",
    "loadout": {
      "factory_version": "b45f445",
      "flight": {
        "short":  { "id": "...", "archetype": "RCSLiquidChemical", "name": "...", /* full engine fields */ },
        "medium": null,
        "far":    null
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

## 5. DB Changes

The Phase 2 `ships` table (`id`, `player_id`, `loadout`, `state`, `transform`, `status`, `created_at`) already covers what we need. Phase 3 adds nothing structural — `loadout` just stops being `'{}'` once `/generate` is hit.

One small addition:

| Change | Notes |
|---|---|
| `ships.factory_version TEXT NULL` | Records which build of the factory produced the ship. Lets us reason about ships generated before a later factory change. |

Snapshotting full system objects inside `loadout` (rather than referencing archetypes by name and re-resolving at read time) means an archetype change never silently mutates an existing player's ship. Archetypes are generator *inputs*, not runtime lookups.

---

## 6. Frontend Changes

**None in Phase 3.** `/generate` is a manual endpoint for inspecting factory output; the frontend does not consume the new `loadout` shape this phase. Frontend work to render the rolled engine is deferred to the next phase.

---

## 7. Testing

- **`factory/...` unit tests** — pure tests, no DB, no HTTP. Fast.
  - Determinism: same seed → same ship.
  - DAG invariants: across many seeds, `IspAtRefPressureSec ≤ IspVacuumSec`, `CanThrottle` matches `MaxThrottle > MinThrottle`, `GimbalRangeDegrees == 0` iff `DryMassKg < GimbalEligibleMassKg`, `InitialAblatorMassKg == 0` iff `CoolingMethod != Ablative`.
  - Archetype reachability: every archetype's full cross-product of enum choices (cooling × mixture × ignition) is reached across N rolls.
  - `Validate()` on every registered archetype passes at package init.
- **Handler tests** — `POST /api/ships/generate` writes a non-empty `loadout`; `GET /api/player` round-trips it.

---

## 8. Open Questions

- **Archetype coverage for v1**: one liquid chemical archetype (Short slot) is the minimum bar. Lean: ship one archetype end-to-end, then go laterally (power/thermal/sensors) rather than pile up propulsion variants.
- **Manufacturer table authoring**: a separate HIL pass with the user — not in scope for the first factory PR, but blocks end-to-end naming (placeholder names are fine until then).
- **Wear-model constants** (`k`, `base_wear`, per-cooling-method coefficients): TBD until the simulation phase starts. Shape is locked; numbers are not.
- **Dispatcher rarity weights**: uniform for v1; rarity tiers for exotic drives punted until there are enough categories to make "rare" meaningful.
- **Phase 3 civilization roster**: exactly one — `GenericCivilization`. Real variety lands in Phase 4 via LLM generation.
- **Phase 4 LLM civ-generation prompt**: needs to be authored. Structured-output call that emits `{description, TechTier, TechProfile}` in a single schema so description and mechanics can't desync.
- **Player-origin civilization**: deferred worldbuilding hook. Wiring is in place (primary-civilization roll is the extension point), but the player-origin source and mission-framing story layer are Phase 4+ concerns.
- **Probe factory in scope?** Listed below as a stretch goal. Same package, new entry point.

---

## 9. Out of Scope for Phase 3

- Medium and Far flight slots (only Short is populated)
- Multi-engine ship assembly beyond the single Short engine
- LLM-driven civilization generation (Phase 4) — Phase 3 uses a single hand-authored `GenericCivilization`
- Non-flight system categories (power, thermal, sensors, structural, life support, propellant tanks)
- Frontend rendering of the rolled engine
- Flight controls / Δv simulation
- Sensor scanning gameplay
- Spacecraft OS CLI
- Multi-player traces / shared universe state
- AI co-pilot
- Economy / resource consumption
- Cockpit instruments tied to specific systems

---

## Phase 3 Definition of Done

- `POST /api/ships/generate` rolls a ship via the factory, persists the `loadout` into the player's active `ships` row, and returns it. Manually hitting this endpoint produces visibly-varying engines across seeds.
- `GET /api/player` round-trips the persisted `loadout`.
- The factory ships the `RCSLiquidChemical` archetype end-to-end: DAG generator, `Validate()` passing at init, cooling/ignition derivation working, Isp two-point interpolation exposed as a method, `HeatToHullW` and `Tick` spec'd but unimplemented.
- Factory unit tests cover determinism and DAG invariants.
- `SystemBase`, `civilizations.go` (single `GenericCivilization` entry), `manufacturers.go` (stub table pointing at the generic civ), `mixtures.go`, and `flight/flight.go` (FlightSlot enum + slot-fill stub) are in place so Phase 4's LLM civ generation and future archetypes are clearly content changes, not scaffolding changes.
