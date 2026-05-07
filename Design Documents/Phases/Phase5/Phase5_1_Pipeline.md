# Civ + Ship Generation Pipeline
*Last updated 2026-05-07*

A walkthrough of how a civilization and a ship come into existence, what files touch each step, and how data flows from HTTP request to JSON response.

---

## Entry point

```
POST /api/ships/generate?seed=<int>          (seed optional — defaults to time.Now)
```

Handler: `server/internal/handlers/ship.go` → `ShipHandler.Generate`.

**Stateless.** Generation runs the full civ pipeline and rolls a civ-aware ship in one shot, returning `{seed, civilization, planet, ship}` as JSON. Nothing is written to the DB. A future `start-game` endpoint will be the only writer of the `runs` table.

503 if `OPENAI_API_KEY` is unset — civ generation is mandatory.

---

## End-to-end flow

```
HTTP POST /api/ships/generate
   │
   ▼
handlers/ship.go      Generate
   │   1. resolve seed (?seed override or time.Now())
   │   2. factory.GenerateCivilization(ctx, llm, seed)   ──► (civ, planet)
   │   3. assembly.GenerateRandomShip(seed, civ)         ──► loadout
   │   4. encode { seed, civilization, planet, ship } as JSON
   ▼
JSON response
```

---

## Civ generation (5 steps)

`factory.GenerateCivilization(ctx, llm, seed)` in `server/internal/factory/civgen.go`:

| # | Step                          | Driver         | File                  |
| - | ----------------------------- | -------------- | --------------------- |
| 1 | Planet                        | procedural     | `factory/planet.go`   |
| 2 | Description + DesignPhilosophy | LLM            | `factory/civgen.go`   |
| 3 | AgeYears                      | procedural     | `factory/civgen.go`   |
| 4 | TechProfile + TechTier        | LLM constrained-choice | `factory/civgen.go` |
| 5 | Name + Flavor                 | LLM            | `factory/civgen.go`   |

The output is a `factory.Civilization` (defined in `factory/civilizations.go`) carrying a `TechProfile` with the dials ship generation will read:

- `DesignPhilosophy` — feeds step 4's prompt; not read by ship gen yet.
- `PreferredCoolingMethods []CoolingMethod`
- `PreferredIgnitionTypes []IgnitionMethod`
- `PreferredMixtureIDs []string`
- `AversionToCryogenics float64` (0..1)
- `RiskTolerance float64` (0..1)
- `ThrustVsIspPreference float64` (-1..+1)
- `FarDriveFamily string`

The civ's name doubles as the ship's manufacturer — every part is stamped with `civ.Name` and a 2–4 letter prefix derived from it (`factory.ShipwrightPrefix`). There is no separate manufacturer registry; in this universe, a company never spans civilizations.

---

## Ship generation

`assembly.GenerateRandomShip(seed, civ)` in `server/internal/factory/assembly/ship.go`:

1. `civBiasFor(civ)` projects the civ's TechProfile + name into a `flight.CivBias` — a flight-package-local read-only struct (`factory/flight/civbias.go`). The indirection exists so `flight` doesn't have to import `factory` (which would close an import cycle).
2. Iterate `FlightSlot{Short, Medium, Far}`. For each slot, call `flight.GenerateForSlot(slot, bias, rng)`.

### `flight.GenerateForSlot`

```
slotRegistry[slot]
   │
   ▼
filter by archetype.minTechTier   ──► civ.TechTier (3 if civ == nil)
   │
   ▼
weighted archetype pick:
   weight = rarity
         × (1 + 0.8 * (1 - |ThrustVsIspPreference - thrustIspBias|/2))   ← thrust↔Isp proximity
         ^ (1 + (1 - 2*RiskTolerance))                                   ← risk sharpening
         × 1.5  if civ.PreferredCoolingMethods overlaps archetype's      ← cooling overlap
   floor at 0.01
   │
   ▼
archetype.generate(civ, rng)  ── liquid.go::GenerateLiquidChemicalEngine
                              └─ far.go::GenerateRelativisticDrive
```

### Per-archetype generator

`factory/flight/liquid.go` and `far.go`. The DAG is documented in liquid.go; the civ-aware bits are:

- **Manufacturer stamp**: every part gets `ManufacturerName = civ.Name` and a serial number `{prefix}-{archetypeShortCode}-{NNNN}` via `factory.PartSerial`.
- **Health**: `rollHealth(range, tier, civ, rng)` shifts the lower bound by `RiskTolerance`. risk=0 → always rolls top of the range; risk=1 → full range.
- **Mixture**: `pickMixture(allowed, civ, rng)` weights each candidate by:
  - 1.0 baseline
  - × 3.0 if `PreferredMixtureIDs[m.ID]`
  - × `(1 - AversionToCryogenics)` if `m.Cryogenic`
  - × 1.5 if the mixture's derived ignition matches `PreferredIgnitionTypes`
  - floor at 0.05

`nil` civ short-circuits every civ-aware path back to a tier-3 baseline with no bias and a generic shipwright stamp.

---

## File map

| Concern                     | File(s)                                                           |
| --------------------------- | ----------------------------------------------------------------- |
| HTTP entry                  | `handlers/ship.go`, `handlers/civilization.go`                    |
| Civ pipeline                | `factory/civgen.go`                                               |
| Civ + planet types          | `factory/civilizations.go`, `factory/planet.go`                   |
| Mixture + ignition types    | `factory/mixtures.go`                                             |
| Resource types + enums      | `factory/resources.go`                                            |
| Manufacturer naming         | `factory/naming.go`                                               |
| Other enums                 | `factory/enums.go`                                                |
| Ship orchestration          | `factory/assembly/ship.go`                                        |
| Flight dispatcher           | `factory/flight/flight.go`                                        |
| Civ→flight bridge           | `factory/flight/civbias.go`                                       |
| Liquid archetype type + gen | `factory/flight/liquid.go`                                        |
| Far archetype type + gen    | `factory/flight/far.go`                                           |
| **Authored data**           | `factory/content/*.go`                                            |
| LLM client                  | `internal/llm/...`                                                |
| Schema (`runs` table)       | `internal/db/db.go`                                               |
| Wiring at startup           | `cmd/server/main.go`                                              |

### Authored data (factory/content/)

Every catalog entry — every resource, mixture, ignition config, and flight archetype — lives in one folder. Each file is a place a content author goes to add new entries; init() in each file calls the appropriate `Register*` function on `factory` or `factory/flight`.

| File                                  | Adds to                                |
| ------------------------------------- | -------------------------------------- |
| `content/resources.go`                | Resource vars (H2O_ICE, SPARK_RESOURCE, ...) |
| `content/mixtures.go`                 | Mixture + IgnitionConfig vars; `factory.RegisterMixture` / `factory.RegisterIgnitionConfig` |
| `content/archetypes_liquid.go`        | LiquidChemicalArchetype vars; `flight.RegisterLiquidArchetype` |
| `content/archetypes_far.go`           | RelativisticDriveArchetype vars; `flight.RegisterRelativisticArchetype` |

`cmd/server/main.go` adds a blank import (`_ "...factory/content"`) so Go's init order guarantees registration before the first request. Tests do the same via per-package `setup_test.go` files.

---

## Determinism

- **Same `(seed, civ)` → same ship.** Step 1 (Planet) and step 3 (Age) are deterministic per seed; steps 2 / 4 / 5 are LLM-driven and not deterministic. Once a civ is fixed, ship rolls against it are deterministic per seed.
- **`?seed=` override** controls both the civ-gen seed (where step 1's planet + step 3's age land) and the ship roll. The LLM steps remain non-deterministic across calls even with the same seed.
- **`FactoryVersion`** (`assembly/ship.go`) tags every generated loadout. Bump it when generation behavior changes invalidate older ships. Currently `phase5_1-v1`.

---

## Where to add new content

Any time you want to add an authored entry, the destination is **`factory/content/`**:

- **New flight archetype** → `content/archetypes_liquid.go` (or `archetypes_far.go`). Declare a struct value; add a `flight.RegisterLiquidArchetype(...)` (or `RegisterRelativisticArchetype`) call to that file's `init()`. Set `Rarity` and `ThrustIspBias`.
- **New mixture** → `content/mixtures.go`. Declare the `factory.Mixture{}` value and add a `factory.RegisterMixture(&...)` call to `init()`. Non-hypergolic non-synthetic mixtures need an `Ignition` config or they'll log a warning.
- **New resource** → `content/resources.go`. Declare the `*factory.Resource` value. Resources don't have a registry — they're referenced by direct var, so just declaring it is enough.
- **New ignition config** → `content/mixtures.go`. Declare it and add a `factory.RegisterIgnitionConfig(&...)` call to `init()`.

**New TechProfile dial** (cross-cutting) → add the field on `factory.TechProfile`, project it into `flight.CivBias` (`assembly/ship.go::civBiasFor`), then read it in `flight.GenerateForSlot` or the per-archetype generator.

---

## What's NOT here yet

- **Persistence.** `POST /api/ships/generate` is stateless. The `runs` table exists in the schema (`db/db.go`) but is unused; the future start-game endpoint will be the writer.
- **Player accounts.** Single implicit player for now.
- **Per-civ manufacturer rosters.** One civ = one shipwright = the civ's name. If the universe ever needs sub-brands, this is where to add them.
- **Civ pregeneration / catalog.** Every call hits the LLM; no batch pre-rolling yet.
