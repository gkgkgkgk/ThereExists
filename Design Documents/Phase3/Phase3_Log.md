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

