# Phase 3 Implementation — The Factory
*2026-04-18*

## Purpose
Translate `Phase3_Plan.md` into a concrete, commit-by-commit implementation plan detailed enough that a developer (or AI agent) with no prior context on this project can execute the phase end-to-end. Do not duplicate design rationale that already lives in the Plan doc — reference it. This doc answers **what code lands in what order and why**.

---

## Prerequisites & starting state

The repo's actual state diverges from what `Phase3_Plan.md` assumes. Do not take the Plan's "Phase 2 already inserts an empty ship" statement at face value — it isn't true. Phase 3 owns that groundwork.

**What exists today (branch `phase-2`, tip `b45f445`):**
- `server/cmd/server/main.go` — HTTP server wiring, CORS, `GET /api/player`, `GET /api/health`.
- `server/internal/db/db.go` — `Connect` + `Migrate`. `Migrate` creates only the `players` table (`id UUID PK`, `seed INTEGER`, `created_at`, `last_seen_at`). **No `ships` table yet.**
- `server/internal/handlers/player.go` — `PlayerHandler.GetPlayer`. Looks up by `?id=<uuid>` or creates a new player row. Returns `{id, seed}`. **Does not touch ships.**
- Client (`client/`) — Three.js scene with cockpit + orbit cameras (Phase 2 output). Consumes `GET /api/player` for `{id, seed}` only. Does not render ship loadout.

**Phase 3 must therefore:**
1. Create the `ships` table and the empty-ship-on-player-creation flow (groundwork the Plan assumed existed).
2. Build `internal/factory/` per the Plan, including civilizations/manufacturers/mixtures scaffolding and the liquid chemical generator.
3. Add `POST /api/ships/generate` as a manual-only endpoint that rolls a ship via the factory and persists the result.

**Out-of-scope for Phase 3** (repeat from Plan §9 for emphasis — do not implement):
- Medium + Far flight slots. Only Short is populated.
- LLM civilization generation. A single hand-authored `GenericCivilization` covers Phase 3.
- Non-flight system categories (power, thermal, sensors, …).
- Frontend consumption of the new loadout shape. `/generate` is manual-only; the client does not need to change in Phase 3.
- `Tick` wear simulation. `HeatToHullW`, `Tick` are defined as methods on the engine struct but either return zero / panic with `"not implemented in phase 3"` — the shape must exist for Phase 4 to fill in.

---

## Branch strategy

- Base: branch off `phase-2` into a new `phase-3` branch.
- Each commit below is a real commit on `phase-3`. Keep them small and atomic — one logical concern per commit, as listed.
- Do not merge `phase-3` into `main` until manual verification (§ "Post-phase verification") passes.

---

## Commit-by-commit plan

### Commit 1 — `db: add ships table`
**Intent.** Create the `ships` table so the factory has somewhere to write. This is the "Phase 2 behavior" the Plan assumed existed. Nothing in this commit is Phase 3 logic — it is the missing groundwork.

**Files modified.**
- `server/internal/db/db.go` — extend `Migrate` with a second `CREATE TABLE IF NOT EXISTS ships ...` statement, executed after the `players` create.

**Schema.**
| Column | Type | Notes |
|---|---|---|
| `id` | `UUID PRIMARY KEY DEFAULT gen_random_uuid()` | |
| `player_id` | `UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE` | |
| `loadout` | `JSONB NOT NULL DEFAULT '{}'::jsonb` | Populated by factory; starts empty. |
| `state` | `JSONB NOT NULL DEFAULT '{}'::jsonb` | Reserved for runtime mutable state (fuel, sensor cache). Unused in Phase 3. |
| `transform` | `JSONB NOT NULL DEFAULT '{}'::jsonb` | Reserved for position/orientation. Unused in Phase 3. |
| `status` | `TEXT NOT NULL DEFAULT 'active'` | `'active'` / `'derelict'`. Only `'active'` is used in Phase 3. |
| `factory_version` | `TEXT` | Nullable. Set when `/generate` runs. |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |

Also add a helper index: `CREATE INDEX IF NOT EXISTS ships_player_active ON ships (player_id) WHERE status = 'active';`. One active ship per player in Phase 3; the partial index makes the active-ship lookup in commit 9 cheap.

**Gotchas.** Run migrations end-to-end on a local Postgres to confirm the two `CREATE TABLE IF NOT EXISTS` statements work in a single `ExecContext` call (Postgres supports multiple semicolon-separated statements). If not, split into two calls.

---

### Commit 2 — `players: insert empty ship on new-player creation`
**Intent.** Every new player gets exactly one `active` ship row with empty loadout. This matches the Plan's §4 statement that player creation inserts an empty ship — now made real.

**Files modified.**
- `server/internal/handlers/player.go`:
  - In the "new player" branch (after the `INSERT INTO players`), add an `INSERT INTO ships (player_id, status) VALUES ($1, 'active')`. Run both inserts in the same `sql.Tx` so a failure on the ship insert doesn't leak an orphan player.
  - Add `ShipID string` to `PlayerResponse`. Populate it from the ship insert's `RETURNING id`.
  - Returning-player branch: after the `UPDATE ... RETURNING seed`, issue a `SELECT id FROM ships WHERE player_id = $1 AND status = 'active' LIMIT 1` to get the ship ID for the response.

**Gotchas.**
- Pre-existing player rows (if any exist in dev) will have no ship row and the returning-player branch will fail the SELECT. Either wipe dev DB, or make the SELECT tolerant and back-fill a ship row if missing (defensive). Lean: wipe dev DB — Phase 3 is pre-launch.
- Keep the existing `rand.NewSource(time.Now().UnixNano())` seeding alone. Factory seeding is a separate concern in commit 9.

---

### Commit 3 — `factory: package skeleton + enums + SystemBase`
**Intent.** Stand up the empty-but-compiling factory package so later commits each add one concern. No logic here — just types and file structure.

**Files created.**
- `server/internal/factory/system.go` — defines `SystemBase` struct exactly as specified in Plan §2 ("SystemBase — shared across every system"). No `QualityTier` field.
- `server/internal/factory/enums.go` — defines three enums: `PropellantConfig`, `IgnitionMethod`, `CoolingMethod`. Each with a `String()` method returning canonical lowercase names. These live in the `factory` root because `TechProfile` (in `civilizations.go`) references `CoolingMethod` and `IgnitionMethod`.
- `server/internal/factory/civilizations.go` — empty package-level `var Civilizations = map[string]*Civilization{}` and the `Civilization` + `TechProfile` struct definitions from Plan §2. Populated in commit 4.
- `server/internal/factory/manufacturers.go` — empty `Manufacturers` map + `Manufacturer` struct. Populated in commit 4.
- `server/internal/factory/mixtures.go` — empty `Mixtures` map + `Mixture` struct. Populated in commit 4.
- `server/internal/factory/flight/flight.go` — defines `FlightSlot` enum (`Short`, `Medium`, `Far`) with `String()` + `MarshalText` / `UnmarshalText` (used as a JSON map key in commit 8); `FlightSystem` interface stub (empty method set — see gotchas); `slotRegistry map[FlightSlot][]archetypeEntry` (unexported); and the dispatcher entry point with its **final signature, locked now**: `GenerateForSlot(slot FlightSlot, civID string, rng *rand.Rand) (FlightSystem, error)`. Returns a typed `ErrSlotEmpty` when no archetype is registered for the slot.

**Gotchas.**
- `FlightSlot` deliberately lives in `flight/`, not in the factory root. The factory package imports `flight` (commit 8), so if `FlightSlot` were at the root, `flight/` importing the root for `FlightSlot` would close the cycle. `TechProfile` doesn't need `FlightSlot` — its `FarDriveFamily` is a `string` tag, not a slot value.
- Keep `FlightSystem` interface *empty* in Phase 3. The moment it grows a method, every concrete type must implement it. With only one concrete type we get nothing from adding methods.
- The dispatcher owns **both** archetype selection and manufacturer selection (see commit 8 for why). Its signature takes `civID` (the primary civilization chosen at ship level) and internally: picks an archetype from `slotRegistry[slot]`, then picks a manufacturer via `PickManufacturerForCivilization(civID, archName, rng)`, then calls the archetype's generator. Locking this signature now prevents a mid-phase refactor.

---

### Commit 4 — `factory: hand-authored civilization, manufacturers, mixtures`
**Intent.** Populate the three registries with the minimum content for Phase 3: one civilization, a small manufacturer roster pointing at it, and the mixture table referenced by the Plan.

**Files modified.**
- `server/internal/factory/civilizations.go`:
  - Register exactly one entry: `GenericCivilization` — `TechTier = 3` (middle of a 1–5 scale), `TechProfile` with no strong preferences (all preference lists empty or contain every enum value, `AversionToCryogenics = 0.0`, `FarDriveFamily = ""`). Flavor text: one sentence, placeholder ("A default mid-tier civilization used during Phase 3 development. Phase 4 replaces this with LLM-generated civs.").
- `server/internal/factory/manufacturers.go`:
  - Register 3 placeholder manufacturers, all with `CivilizationID = "GenericCivilization"`. Plausible names (user will replace via HIL pass later — this is scaffolding). Each has a trivial `NamingConvention` like `fmt.Sprintf("%s-%d", arch, rng.Intn(9000)+1000)`. `ArchetypeWeights map[string]float64` can be `nil` (treat nil as "equal weights for all archetypes").
  - Export `PickManufacturerForCivilization(civID string, archName string, rng *rand.Rand) *Manufacturer`. Signature is **archetype-aware**: the dispatcher in `flight.GenerateForSlot` picks the archetype first, then calls this with the archetype name so `ArchetypeWeights` can bias the choice. Implementation: filter manufacturers by `CivilizationID == civID`; weight each by `ArchetypeWeights[archName]` (default 1.0 if map is nil or key missing); sample. Never called from ship-level code — only from the dispatcher.
- `server/internal/factory/mixtures.go`:
  - Register 4 entries: `LOX_LH2` (Bipropellant, Cryogenic, storability -1), `LOX_RP1` (Bipropellant, Cryogenic), `MMH_NTO` (Bipropellant, Hypergolic, storability 3650), `Hydrazine` (Monopropellant, storability 3650). `IspMultiplier` around 1.0 for all — the archetype ranges already encode Isp spread.

**Gotchas.**
- These tables are referenced by tests in commit 10. Any drift in IDs (e.g. renaming `MMH_NTO` → `MMH-NTO`) will break the `RCSLiquidChemical` archetype's `AllowedMixtureIDs` list in commit 6. Keep IDs stable.

---

### Commit 5 — `factory: archetype validation harness`
**Intent.** Before any real archetype lands (commit 6), establish the validation pattern: an `Validate() error` method, an `init()` that iterates a registry of all registered archetypes and panics on error. This way commit 6's `RCSLiquidChemical` plugs into a ready-made guardrail.

**Files created.**
- `server/internal/factory/flight/liquid.go` — create the file with only the structs (`LiquidChemicalArchetype`, `LiquidChemicalEngine`), and a `var registeredArchetypes []LiquidChemicalArchetype` plus an `init()` that iterates and calls `arch.Validate()`. No archetypes registered yet, so `init()` is a no-op. The `Validate()` method is defined but, with no archetypes to check, has no callers yet.

**Validate() logic.**
Scope exactly as Plan §2 "Archetype validation" describes. Concretely, it checks:
- All `[2]float64` range fields have `lo <= hi`.
- `GimbalEligibleMassKg` is within or above `DryMassRange` (not below `DryMassRange.lo`, which would make the gimbal gating dead code).
- `AllowedMixtureIDs` is non-empty and every ID exists in `factory.Mixtures`.
- `AllowedCoolingMethods` is non-empty.
- For every mixture in `AllowedMixtureIDs`: at least one `IgnitionMethod` is derivable from it (Plan §2 Group 7 logic). Prevents "hypergolic-only mixture list paired with no ignition path."
- `HealthInitRange` ⊂ `[0.0, 1.0]`.
- `ReferencePressurePa > 0`.

Return a multi-error with all violations (not just the first) so archetype authors get a complete list on one run.

---

### Commit 6 — `factory: RCSLiquidChemical archetype`
**Intent.** Land the one Phase 3 archetype. Instance generation does not exist yet — this commit only declares the data.

**Files modified.**
- `server/internal/factory/flight/liquid.go`:
  - Add `var RCSLiquidChemical = LiquidChemicalArchetype{...}` with the exact values from Plan §2 "Example archetype (v1)". Append it to `registeredArchetypes` in the same file's `init()` so `Validate()` runs at package load.
  - Add a parallel `init()`-registered entry into `flight.slotRegistry[Short]` so commit 7's `GenerateForSlot(Short, ...)` can find it. The entry is `{archetypeName: "RCSLiquidChemical", generator: GenerateLiquidChemicalEngine_placeholder}` — the generator is a placeholder that returns `errors.New("generator not implemented — see commit 7")`. This keeps the commit green without the generator yet.

Also add a small helper method on the engine struct (definition lands here alongside the struct; the generator in commit 7 is the caller): `(e *LiquidChemicalEngine) HasRestartsRemaining() bool { return e.MaxRestarts < 0 || e.RestartsUsed < e.MaxRestarts }`. Convention: `MaxRestarts == -1` means unlimited. Centralizing this prevents the check from being reinvented (incorrectly) in flight-logic code later.

**Gotchas.**
- `Validate()` runs at `init()`. If the archetype is wrong, server startup panics. Good — that's the guardrail. Run `go test ./...` after this commit to confirm init doesn't blow up.
- `flight/liquid.go` depends on `factory.Mixtures` being populated first. Since `flight` imports `factory`, Go guarantees `factory`'s `init()` runs before `flight`'s. No test needed — this is a language guarantee, not a convention.

---

### Commit 7 — `factory: liquid chemical generator (the DAG)`
**Intent.** Implement the Plan §2 generation-order DAG as `GenerateLiquidChemicalEngine(arch, rng)`. Replace the placeholder from commit 6.

**Files modified.**
- `server/internal/factory/flight/liquid.go`:
  - Implement `GenerateLiquidChemicalEngine(arch LiquidChemicalArchetype, ctx factory.GenContext) (*LiquidChemicalEngine, error)` following the 9 groups in Plan §2. `ctx` carries `ManufacturerID` and `Rng` (see commit 8 for the struct definition). Concrete per-group recipes:
    1. **Group 0 — SystemBase fields, enumerated explicitly**:
       - `ID = uuid.New().String()`
       - `ArchetypeName = "RCSLiquidChemical"` (literal — matches the name used in `slotRegistry`)
       - `SerialNumber = Manufacturers[ctx.ManufacturerID].NamingConvention(ctx.Rng, "RCSLiquidChemical")`
       - `ManufacturerID = ctx.ManufacturerID`
       - `Health`: sample uniformly from `HealthInitRange`, then narrow by `TechTier ∈ {1..5}` — recipe: `narrowedLo = lo + (tier-1)/4 * (hi-lo)*0.5`, `narrowedHi = hi`. Tier 1 gets the full range; tier 5 gets only the top half. Monotonic in tier.
    2. **Group 1 — ChamberPressureBar**: uniform over `ChamberPressureRange`.
    3. **Group 2 — Cooling**: filter `AllowedCoolingMethods` by chamber pressure; package-level constants `AblativePressureCeilingBar = 150.0`, `RadiativePressureCeilingBar = 40.0`. Pick uniformly from survivors. If `Ablative`, roll `InitialAblatorMassKg` uniform over archetype's ablator range.
    4. **Group 3 — Isp / Thrust** (concrete recipes so values don't escape archetype ranges):
       - `pNorm = (ChamberPressureBar - ChamberPressureRange.lo) / (ChamberPressureRange.hi - ChamberPressureRange.lo)`, clamped to `[0,1]`.
       - `IspVacuumSec`: sample `u` uniform in `[0,1]`, then `bias = u + pNorm*(1-u)` (pulls toward 1 as pressure grows); `IspVacuumSec = IspVacuumRange.lo + bias*(IspVacuumRange.hi - IspVacuumRange.lo)`. Guaranteed ∈ range.
       - `IspAtRefPressureSec`: uniform over `[IspAtRefPressureRange.lo, min(IspAtRefPressureRange.hi, IspVacuumSec)]`. Guaranteed ∈ archetype range AND ≤ vacuum Isp.
       - `ThrustVacuumN = logUniform(ThrustVacuumRange.lo, ThrustVacuumRange.hi, rng)`.
    5. **Group 4 — DryMassKg**: `tNorm = (ThrustVacuumN - ThrustVacuumRange.lo) / (ThrustVacuumRange.hi - ThrustVacuumRange.lo)`, clamped. Sample `base = logUniform(DryMassRange.lo, DryMassRange.hi, rng)`. Final: `DryMassKg = min(DryMassRange.hi, base * (1 + 0.5*tNorm))`. Clamp guarantees we stay in archetype range.
    6. **Group 5 — Gimbal**: if `DryMassKg >= GimbalEligibleMassKg`, sample `GimbalRangeDegrees` uniform over archetype's gimbal range; else 0.
    7. **Group 6 — Power**: `IgnitionPowerW` and `OperatingPowerW` independent uniform rolls over their respective archetype ranges.
    8. **Group 7 — Mixture + propellant + ignition**: pick `MixtureID` uniformly from `AllowedMixtureIDs`; derive `PropellantConfig` from `Mixtures[MixtureID].Config`; derive `IgnitionMethod` from mixture flags (hypergolic → `Hypergolic`; monopropellant → `Catalytic`; else uniform over `{Spark, Pyrotechnic}`).
    9. **Group 8 — Burn limits**: `MaxContinuousBurnSeconds` uniform over archetype range, multiplied by 0.5 if `CoolingMethod == Ablative`. `MaxRestarts = arch.MaxRestarts` (scalar, copied). `MinThrottle`/`MaxThrottle` sampled honoring archetype range; set `CanThrottle = (MaxThrottle > MinThrottle)`. `RestartTemperatureCeilingK`: sample uniform over archetype range, then subtract `pNorm * 0.2 * (hi-lo)` (higher pressure → tighter ceiling). Clamp to archetype range.
    10. **Group 9 — Zero state**: `RestartsUsed = 0`, `IsFiring = false`, `CurrentTemperatureK = 293.15` (ambient).
  - Implement `(e *LiquidChemicalEngine) IspAt(ambientPa float64) float64` exactly as in Plan §2.
  - Implement `(e *LiquidChemicalEngine) HeatToHullW(throttle float64) float64` — returns `0.0` in Phase 3. Method exists so callers don't break when the real formula lands.
  - Implement `(e *LiquidChemicalEngine) Tick(dt, throttle float64)` — `panic("Tick not implemented in Phase 3")`. Any accidental call during dev fails loudly.
  - Add a small helper for log-uniform sampling: `logUniform(lo, hi float64, rng *rand.Rand) float64`. Put it in a `sampling.go` file in the factory root package so other categories can reuse it.

**Gotchas.**
- The superlinear-thrust-from-chamber-pressure and similar biases are *prose* in the Plan; you pick reasonable functional forms. Document each choice in a comment referencing the Plan group it implements.
- Deterministic seeding: the generator takes `*rand.Rand`, so determinism is the caller's responsibility. Do not use `math/rand`'s global functions anywhere inside the generator or helpers.

---

### Commit 8 — `factory: GenerateRandomShip entry point`
**Intent.** Glue: a single top-level function that picks civilization + manufacturers, calls `flight.GenerateForSlot(Short, ...)`, and returns a serializable `ShipLoadout`. This is what the handler in commit 9 calls.

**Files created.**
- `server/internal/factory/gencontext.go`:
  - Define `GenContext struct { ManufacturerID string; Rng *rand.Rand }`. Threaded from dispatcher into category generators. Living in the factory root lets the `flight/` subpackage import it without cycling.
- `server/internal/factory/ship.go`:
  - Define `ShipLoadout` struct: `FactoryVersion string` + `Flight map[flight.FlightSlot]any`. Value is `any` because `FlightSystem` is empty and we want concrete types to JSON-marshal cleanly; a tagged interface would force a wrapper. Using `any` is a Phase-3 shortcut; tighten when `FlightSystem` grows methods.
  - `GenerateRandomShip(seed int64) *ShipLoadout`:
    1. Construct `rng := rand.New(rand.NewSource(seed))`.
    2. Pick primary civilization. Phase 3: always `"GenericCivilization"`.
    3. Initialize `loadout.Flight = map[flight.FlightSlot]any{}` — an allocated, empty map.
    4. For each of `{Short, Medium, Far}`:
       - Call `flight.GenerateForSlot(slot, primaryCivID, rng)`. **Ship-level code does NOT pick the manufacturer.** The dispatcher picks the archetype first (from `slotRegistry[slot]`), then calls `PickManufacturerForCivilization(civID, archName, rng)` with the known archetype name so `ArchetypeWeights` actually work, builds a `GenContext`, and hands it to the archetype's generator.
       - On `ErrSlotEmpty`, explicitly set `loadout.Flight[slot] = nil`. The nil assignment (not omission) is required for the JSON marshaler to emit `"medium": null` instead of dropping the key. In Phase 3, Medium and Far always return `ErrSlotEmpty`.
       - Otherwise store the returned system.
    5. Populate `FactoryVersion` from a package-level `const FactoryVersion = "phase3-v1"`. Bump the suffix whenever generation behavior changes in a way that invalidates old persisted loadouts. (Phase 4 upgrade path: replace constant with `runtime/debug.ReadBuildInfo()` VCS stamp once builds are reproducible — not worth it now.)
  - Register a `MarshalJSON` on `ShipLoadout` that emits the shape shown in Plan §4 response example: `{"factory_version": "...", "flight": {"short": {...}, "medium": null, "far": null}}`. Slot keys are lowercase via `FlightSlot.MarshalText` from commit 3.

**Gotchas.**
- Ship-level code is intentionally thin: pick civilization, loop slots, dispatch. All archetype/manufacturer coordination lives inside `flight.GenerateForSlot`. This is the inversion from the earlier draft where ship-level picked the manufacturer first — which broke `ArchetypeWeights` because the archetype name wasn't known yet.
- The `any`-typed Flight map is a deliberate Phase 3 shortcut. Add a `// TODO(phase-4): replace with FlightSystem interface once it has methods.` comment.
- `loadout.Flight[Medium] = nil` vs. omission: Go's JSON encoder emits `null` for explicit nil map values but drops missing keys entirely. The frontend (Phase 4) will expect the keys to exist — make the null assignment explicit.

---

### Commit 9 — `handlers: POST /api/ships/generate`
**Intent.** Manual debug endpoint. Caller posts a player ID; server rolls a ship and persists loadout.

**Files created.**
- `server/internal/handlers/ship.go`:
  - `ShipHandler` struct holding `*sql.DB`.
  - `Generate(w, r)` handler:
    1. Parse player ID from query param `?player_id=<uuid>` (simpler than a JSON body for a manual endpoint).
    2. Look up the player's active ship: `SELECT id FROM ships WHERE player_id = $1 AND status = 'active'`. 404 if not found.
    3. Derive seed: `SELECT seed FROM players WHERE id = $1`. Use `int64(seed)` as the factory seed. This makes `/generate` deterministic per player — hitting it twice produces the same ship. Acceptable for Phase 3 debug; future phases may move to a per-ship seed.
    4. Call `factory.GenerateRandomShip(seed)`.
    5. Marshal loadout to JSON, `UPDATE ships SET loadout = $1, factory_version = $2 WHERE id = $3`.
    6. Respond `200 OK` with the loadout JSON.

**Files modified.**
- `server/cmd/server/main.go`:
  - Construct `ShipHandler`, register `mux.HandleFunc("POST /api/ships/generate", sh.Generate)`.

**Gotchas.**
- The deterministic-per-player seed means re-rolling requires changing input. Easy Phase 3 workaround: accept an optional `?seed=<int>` query param that overrides the player's seed. Default to the player's seed when absent. Lets the user hit the endpoint with different seeds without creating new players.
- CORS allows `POST`. Good. No auth — this is dev-only.

---

### Commit 10 — `factory: unit tests`
**Intent.** Lock in DAG invariants and determinism so later refactors don't silently regress the generator.

**Files created.**
- `server/internal/factory/flight/liquid_test.go`:
  - `TestDeterminism` — same seed, same archetype → identical engine (deep equality).
  - `TestDAGInvariants` — loop 1000 seeds; for each generated engine assert:
    - `IspAtRefPressureSec <= IspVacuumSec`
    - `CanThrottle == (MaxThrottle > MinThrottle)`
    - `(GimbalRangeDegrees == 0) == (DryMassKg < archetype.GimbalEligibleMassKg)`
    - `(InitialAblatorMassKg > 0) == (CoolingMethod == Ablative)`
    - `Health ∈ [0, 1]`, `RestartsUsed == 0`, `IsFiring == false`
  - `TestReachability` — per-archetype with a **hardcoded expected reachable set**, not a runtime-computed "post-pressure-filter" set (that would just re-run the generator logic against itself and pass by construction). For `RCSLiquidChemical`, hardcode the expected reachable cooling methods and ignition methods given its archetype values, then assert across N=10_000 seeds that each expected value appears at least once. If the archetype values change, the test must be updated — that's the point: the test is a tripwire for accidental distribution shifts.
- `server/internal/factory/factory_test.go`:
  - `TestValidateAllArchetypes` — iterates every registered archetype across all categories and calls `Validate()`. Currently just exercises `RCSLiquidChemical`, but costs nothing to write and catches future misconfiguration.

**Gotchas.**
- Tests should not hit the DB or HTTP. They are pure-Go, sub-second.

---

### Commit 11 — `handlers: /api/ships/generate integration test`
**Intent.** End-to-end confirmation: `POST /generate` writes a non-empty `loadout`, and a subsequent `GET /api/player` round-trips the ship ID.

**Files created.**
- `server/internal/handlers/ship_test.go`:
  - Standard integration pattern (if the project doesn't have one yet, follow Phase 2's handler test conventions — if none exist, document that this commit introduces the pattern). Spin up a test Postgres (via `docker-compose.test.yml` or a CI-provided instance), run migrations, create a player row + active ship row, POST `/api/ships/generate?player_id=<id>`, assert:
    - Response is 200 + JSON with non-empty `flight.short`.
    - DB row: `ships.loadout ->> 'factory_version'` is non-null; `ships.loadout -> 'flight' -> 'short'` is non-null JSON object.
    - `GET /api/player?id=<id>` still returns the same `ship_id`.

**Gotchas.**
- If no Postgres-in-CI exists, this commit can skeleton the test with a `t.Skip("needs DB setup")` and a TODO. Don't block Phase 3 on CI infrastructure that isn't in place.

---

## Post-phase verification (manual)

Run once before merging `phase-3` → `main`:

1. Fresh DB: drop and re-create the Postgres schema so migrations run cleanly from scratch.
2. Start server. Hit `GET /api/player` → note `{id, seed, ship_id}`.
3. `curl -X POST "http://localhost:8080/api/ships/generate?player_id=<id>"` → expect 200 with a non-empty `flight.short` engine.
4. Hit it again with `?seed=42` override — expect a visibly different engine.
5. `psql` the DB, `SELECT loadout FROM ships WHERE player_id = '<id>'` → confirm `factory_version` field, `flight.short` populated, `flight.medium` and `flight.far` are `null`.
6. Repeat step 3 with the same seed twice — confirm bit-identical loadout JSON.
7. Server restart — repeat step 3, confirm results still match (determinism across process restarts).
8. Run `go test ./...` — all green.

If any of the above fails, fix before writing the Phase 3 Summary doc.

---

## Deferred-but-spec'd items (do not build in Phase 3)

These exist as **stubs or empty methods** after Phase 3 — listed here so the next phase's author sees them as extension points, not missing scaffolding:

- `(*LiquidChemicalEngine).HeatToHullW` — returns `0.0`. Real formula in Phase 4.
- `(*LiquidChemicalEngine).Tick` — panics. Real wear/thermal integration in Phase 4.
- `flight.FlightSystem` interface — empty. Grows methods once a second flight category exists.
- `factory.GenerateRandomShip` — single civilization, only Short slot. Extends to multi-civ + all-slots in Phase 4 once LLM civ generation + Medium/Far archetypes land.
- `ShipLoadout.Flight` uses `map[FlightSlot]any`. Tighten to `map[FlightSlot]FlightSystem` when the interface has methods.

---

## Summary of what lands

After Phase 3, the repo has: a `ships` table; an empty-ship-on-player-creation flow; a working `internal/factory/` package with civilizations, manufacturers, mixtures, and a complete liquid chemical engine generator (DAG-ordered, validated, deterministic); a manual `POST /api/ships/generate` endpoint that rolls and persists a ship; and unit + integration tests covering determinism, DAG invariants, and round-trip persistence. Players' ship rows go from literal `'{}'` loadouts to fully specified (but single-slot) flight systems. The frontend is untouched.
