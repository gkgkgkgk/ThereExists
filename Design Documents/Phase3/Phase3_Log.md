# Phase 3 Build Log

Running log of the Phase 3 implementation. One entry per commit (or notable decision). Paired with `Phase3_Implementation.md` — that doc is the plan; this is what actually happened.

---

## 2026-04-19 — kickoff

Branched off `phase-3` (tip `9be7593`, design docs). Starting from the commit plan in `Phase3_Implementation.md`. Aiming to land commits 1–3 (groundwork + package skeleton) and then pause for review before the DAG lands.

Repo state check before starting:
- `server/internal/db/db.go` migrates `players` only — no `ships` table yet, matching the Impl doc's "starting state" section.
- `handlers/player.go` returns `{id, seed}` only — no ship_id, no ship insert.
- No `internal/factory` package yet.

So commit 1 is indeed the missing Phase 2 groundwork, exactly as the Impl doc warns.

### Commit `7985b77` — `db: add ships table`
Done. Schema matches Impl §Commit 1 (id, player_id FK, loadout/state/transform JSONB, status, factory_version, created_at) plus the partial index on active ships. Single `ExecContext` with multiple `;`-separated statements works on Postgres — no need to split.

### Commit `3949af7` — `docs: add Phase 3 build log`
Added this file + the "Log Design Documents" section in DesignDocDesignDoc.md (pre-existing uncommitted edit from the user that defines what this log is). Also added `.claude/` to `.gitignore` — the harness's local settings file is not project state.

### Commit `8af722e` — `players: insert empty active ship on player creation`
Phase 2 groundwork. `PlayerHandler.GetPlayer`:
- New-player branch now wraps `INSERT INTO players` + `INSERT INTO ships (..., status='active') RETURNING id` in a single `sql.Tx` so a ship-insert failure doesn't leak an orphan player.
- Returning-player branch does a follow-up `SELECT id FROM ships WHERE player_id = $1 AND status = 'active' LIMIT 1`. Pre-existing dev rows without a ship will 500; Impl doc accepts that (wipe dev DB).
- `PlayerResponse` gains `ship_id`. Refactored the handler into three small funcs for readability.

### Commit (next) — `factory: package skeleton + enums + SystemBase`
Stood up `server/internal/factory/`:
- `system.go` — `SystemBase` struct (no QualityTier field).
- `enums.go` — `PropellantConfig`, `IgnitionMethod`, `CoolingMethod`, each with `String()` + `MarshalText`.
- `civilizations.go` / `manufacturers.go` / `mixtures.go` — struct definitions + empty registry maps, populated in commit 4.
- `flight/flight.go` — `FlightSlot` enum (Short/Medium/Far) with Marshal/Unmarshal text; empty `FlightSystem` interface; `slotRegistry` (unexported); `GenerateForSlot(slot, civID, rng)` dispatcher with locked signature.

One divergence from the Impl doc worth flagging: the Impl doc implies `flight.GenerateForSlot` directly calls `factory.PickManufacturerForCivilization`. But `flight/` imports can't see the `factory` root (that'd cycle once `factory.ship.go` imports `flight/`). Solved with a `SetManufacturerPicker(ManufacturerPicker)` injector — factory root wires it up at startup. Signature from the ship-level caller's view is unchanged.

`register()` in flight.go is unused until commit 6 registers `RCSLiquidChemical`. Build passes; the lint warning is expected.

### Commit `b5fccf5` — `factory: populate civilization, manufacturer, mixture registries`
Impl commit 4. `GenericCivilization` (Tier 3, neutral `TechProfile`). Three placeholder manufacturers — Kirov Rocketworks, Helios Propulsion, Triton Dynamics — all pointing at the generic civ. Each has a `NamingConvention` that produces serials like `KR-RCS-LC-3987`. Mixture table: `LOX_LH2`, `LOX_RP1` (cryogenic bipropellants), `MMH_NTO` (hypergolic bipropellant), `Hydrazine` (monopropellant). IDs frozen — `RCSLiquidChemical` references `MMH_NTO` and `Hydrazine` by name.

Exported `factory.PickManufacturer` (weighted by `ArchetypeWeights`, uniform default). Originally planned to wire it into `flight.SetManufacturerPicker` from factory's `init()`, but that creates a `factory → flight → factory` import cycle (flight imports factory for `Mixtures`/`CoolingMethod`). Moved the wiring to `main.go` — factory stays leaf of the factory → flight edge.

### Commit `9eca35a` — `factory: LiquidChemicalArchetype, engine struct, Validate, RCSLiquidChemical`
Bundled Impl commits 5 + 6 (too small to split).
- `LiquidChemicalArchetype` and `LiquidChemicalEngine` structs (field order matches the DAG groups).
- `HasRestartsRemaining()` method centralises the `MaxRestarts == -1` sentinel.
- `Validate()` runs on every registered archetype at `init()`. Checks only what the DAG cannot structurally guarantee: range monotonicity, gimbal-gating reachability, non-empty mixture/cooling lists, every `AllowedMixtureIDs` entry exists in `factory.Mixtures`, `HealthInitRange` ⊂ [0, 1], `ReferencePressurePa > 0`.
- `RCSLiquidChemical` values copied from Plan §2 "Example archetype (v1)", registered via `registerLiquidArchetype`.
- Generator is a named placeholder (`makeLiquidGenerator` var, returns `not implemented` error) so this commit compiles. Commit 7 reassigns it.

### Commit (next) — `factory: liquid chemical DAG generator`
Impl commit 7 landed. Files:
- `sampling.go` (`LogUniform`, `Uniform`, `Clamp01`) in the factory root so any future category can share it.
- `gencontext.go` (`GenContext { ManufacturerID, Rng }`).
- `flight/liquid_generator.go` — the 9-group DAG, plus `IspAt`, `HeatToHullW` (returns 0.0 — Phase 4), `Tick` (panics — Phase 4), and the small helpers (`filterCoolingByPressure`, `deriveIgnition`, `rollHealth`).

Pressure thresholds for cooling filtering (`AblativePressureCeilingBar = 150`, `RadiativePressureCeilingBar = 40`) live as package constants, not per-archetype — they're physics, not flavour.

Init-order quirk: `liquid.go`'s `init()` calls `registerLiquidArchetype` which captures the placeholder `makeLiquidGenerator`. `liquid_generator.go`'s `init()` replaces the placeholder AND calls `rebuildSlotRegistry()` to re-register every archetype with the real generator. Unavoidable given Go's one-init-per-file model and the desire to keep Validate() in the commit that introduced the struct.

Smoke test run (seed 42, `GenericCivilization`): rolled a `KR-RCS-LC-3987` monopropellant Hydrazine engine — 9.2 bar chamber, radiative cooling (sub-40 bar), catalytic ignition, no ablator, no gimbal, on/off throttle, 109s max burn, 293K ambient, all derived correctly from the DAG.


