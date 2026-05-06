# Phase 5.1 Plan — Civ-Aware Ship Generation, Persisted
*2026-05-06*

## Goal
Every player has a unique civilization, generated on first ship-roll, persisted, and used as the bias source for that player's ship generation. The ship factory becomes civ-aware: it reads `Civilization.TechProfile` at each existing slot/archetype/mixture decision point and skews the weights accordingly. Civ generation order does **not** change ship generation order — civs are a passive bag of biases, ship gen keeps its current Short → Medium → Far DAG.

Phase 5.1 lands persistence of civilizations, the player → civ wiring, and the dial readouts in flight generation. It does **not** land cross-civ ship transfer/capture mechanics, manufacturer-from-civ generation (deferred — when a civ spawns its own manufacturer roster, that's the right place to land manufacturer preference), pre-generation of civ catalogs, or any retuning of which civs use which Far drives (still gated by `TechTier` per Phase 4).

---

## 1. Database — `civilizations` table + civ FKs

### Schema
New migration in `server/internal/db/db.go`:

```sql
CREATE TABLE IF NOT EXISTS civilizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    description     TEXT NOT NULL,
    homeworld_desc  TEXT NOT NULL,
    age_years       BIGINT NOT NULL,
    tech_tier       INTEGER NOT NULL,
    flavor          TEXT NOT NULL,
    profile         JSONB NOT NULL,        -- TechProfile blob
    planet          JSONB NOT NULL,        -- the Planet that seeded it
    factory_version TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS civ_id UUID
        REFERENCES civilizations(id) ON DELETE SET NULL;

ALTER TABLE ships
    ADD COLUMN IF NOT EXISTS civ_id UUID
        REFERENCES civilizations(id) ON DELETE SET NULL;
```

### Why both `players.civ_id` and `ships.civ_id`?
A player has *a* civilization (their nationality). A ship has *the civ that built its loadout*. Today they're always equal — the player rolls a ship for their civ. But two future scenarios make ship.civ_id load-bearing: (a) captured ships from other civs, (b) regenerating a player's loadout against a different civ for testing. Adding the column now costs nothing and saves a migration later. Both columns are nullable for backward compatibility with players/ships that exist before Phase 5.1.

### `profile` JSONB shape
The full `TechProfile` struct serialized as JSON. Includes `DesignPhilosophy`, `RiskTolerance`, `ThrustVsIspPreference`, all the preference arrays, and `FarDriveFamily`. Storing as JSONB (not normalized columns) because:
- These fields are read together (always loaded as a unit by ship gen);
- Schema evolution is easier when `TechProfile` grows new dials (which it will);
- We never query *into* the profile from SQL — the ship factory walks the whole struct in Go.

### `planet` JSONB shape
The full `Planet` struct that seeded the civ. Stored so we can re-render or re-display the civ's homeworld without losing the original sample. When the planet-generation phase lands, this becomes a `homeworld_planet_id` FK; until then, embed.

### Backward compatibility
- Existing players (none in dev DBs, possibly some in long-running databases) get `civ_id = NULL`. The provisioning flow generates a civ on first ship-roll attempt.
- Existing ships keep `civ_id = NULL`. Their loadouts remain valid; civ-aware regen happens only when the player rolls a fresh ship.

---

## 2. The Persistence Layer

New file `server/internal/factory/civstore.go` (or extend `db/`; placement decision in §10). Two functions:

```go
func SaveCivilization(ctx context.Context, db *sql.DB, civ *Civilization, planet *Planet, factoryVersion string) error
func LoadCivilization(ctx context.Context, db *sql.DB, id string) (*Civilization, *Planet, error)
```

`Save` marshals `civ.TechProfile` and `planet` to JSON, INSERTs the row, returns the assigned UUID via `civ.ID` mutation (or returns it; signature TBD). `Load` SELECTs and unmarshals. Both are thin — the schema is wide but the marshaling is straightforward.

### Why not in `internal/db/`?
`internal/db/` today is just connection + migrations, no domain knowledge. Putting civilization persistence there leaks `factory.Civilization` into the db package. Cleaner: persistence lives next to the type it persists, in `factory/`. The civstore takes a `*sql.DB` so it doesn't depend on a package-global connection.

---

## 3. Player Provisioning Flow

### Current state
- `GET /api/player` either looks up an existing player or creates one. Creation provisions an empty ship row (no loadout).
- `POST /api/ships/generate?player_id=X` rolls a loadout and persists it onto the player's active ship row. Lazy: player creation is fast, ship gen is the LLM-cost moment.

### Change
Civ generation happens at the same moment as ship generation, not at player creation. Rationale: keeps `GET /api/player` synchronous and cheap; ties the LLM cost to the explicit `/ships/generate` action; lets us iterate civ prompts without spinning up real players.

### New flow inside `POST /api/ships/generate`
1. Look up player → get `civ_id`.
2. **If `civ_id` is null:** generate a civilization (calls `factory.GenerateCivilization` — already exists), persist it via `SaveCivilization`, update `players.civ_id = <new>`. All in one transaction.
3. Load the civilization (either the fresh one or the existing one).
4. Roll the ship via `assembly.GenerateRandomShip(seed, civ)` — new signature, see §4.
5. Persist loadout *and* `ships.civ_id = player.civ_id`.

### Endpoint signature
Stays the same: `POST /api/ships/generate?player_id=X[&seed=N]`. No new query params. Civ generation is invisible to the caller — they get back a loadout, same as before, just one that's now culturally consistent with the player's persisted civ.

### Cost & latency
First ship roll for a player now costs description + tech-profile + name-flavor LLM calls *before* the ship rolls. ~3–5s extra on first call. Subsequent rolls (seed override, regen) skip civ gen because `civ_id` is set. Acceptable for an admin/test endpoint.

---

## 4. Civ-Aware Ship Generation

### Signature change
```go
// Before
func GenerateRandomShip(seed int64) (ShipLoadout, error)

// After
func GenerateRandomShip(seed int64, civ *factory.Civilization) (ShipLoadout, error)
```

`civ == nil` falls back to the current uniform-bias behavior (plus `GenericCivilization` for tier gating). Lets us delete nothing — existing tests and the no-civ path keep working.

### Threading
`assembly.GenerateRandomShip` passes `civ` into `flight.GenerateForSlot(slot, civID, seed, civ)`. The flight dispatcher grows a `*factory.Civilization` parameter and uses it inside the existing weighted-pick loops. No new traversal order, no new steps.

### Tier gating
Already wired via `flight.SetCivTechTierLookup` + Phase 4 commit 6. The new path passes the actual generated civ's tier instead of looking up `GenericCivilization` every time. Far slot continues to gate at tier 5.

---

## 5. The Dials Wired

Each dial is a *weight multiplier* applied at the existing decision points. Order from most-impactful to least.

### `ThrustVsIspPreference` (highest leverage)
Most slots have multiple archetypes (Short: RCA / TCA / PDD; Medium: HPFA / SCTA / RDE / SABRE). Each archetype has a characteristic T/W vs Isp signature. New per-archetype scalar `ThrustIspBias` in `[-1, 1]` declared alongside rarity weights:

- RCA: -0.2 (balanced, slight thrust)
- TCA: +0.0 (balanced)
- PDD: +0.4 (Isp-leaning, electrical)
- HPFA: -0.3 (thrust-leaning storable)
- SCTA: +0.5 (high-Isp mainline)
- RDE: -0.7 (punchy detonation)
- SABRE: +0.2 (efficient biological)
- RBCA: +1.0 (extreme Isp, Far slot)

Pick weight multiplier = `1.0 + 0.8 * (1 - |civ.ThrustVsIspPreference - archetype.ThrustIspBias| / 2)`. Result: a civ at +1.0 (long-burn lover) gets ~1.4× weight on SCTA, ~0.6× on RDE. Clamped to never zero out an option entirely (a far-from-preferred archetype still has a small chance — preferences bias, they don't dictate).

### `RiskTolerance`
Two effects, both via the existing rarity weights:
1. **Multiplies into rarity bias.** Low-tolerance civs sharpen the weights toward common archetypes (`weight^(1 + (1 - risk))`). High-tolerance civs flatten them (`weight^risk`). Result: a 0.1-tolerance civ almost never rolls SABRE; a 0.9-tolerance civ rolls it nearly as often as SCTA.
2. **Seeds the `HealthInitRange` lower bound.** When the archetype rolls `Health`, the lower bound shifts: `effective_min = lerp(archetype.HealthInitRange[1], archetype.HealthInitRange[0], civ.RiskTolerance)`. A 0.0-tolerance civ always starts at top health; a 1.0-tolerance civ accepts the archetype's full range.

### `PreferredMixtureIDs` + `AversionToCryogenics`
Inside the per-slot mixture pick (currently uniform across `archetype.AllowedMixtureIDs`):
- Each mixture's base weight = 1.0.
- If the mixture ID is in `civ.PreferredMixtureIDs`, weight × 3.0.
- If `mixture.Cryogenic == true`, weight × `(1.0 - civ.AversionToCryogenics)`.
- Sample weighted. Floor at 0.05 to keep exotic options non-zero.

### `PreferredCoolingMethods` / `PreferredIgnitionTypes`
Soft archetype-selection bias. After the base archetype weights are computed (rarity × ThrustVsIsp × RiskTolerance), multiply by 1.5 if any of the archetype's `AllowedCoolingMethods` overlap `civ.PreferredCoolingMethods`. Same shape for ignition types via the rolled mixture's `IgnitionMethod`. Soft because civ preferences shouldn't override the physics — a Far drive that needs Magnetic ignition isn't going to switch to Spark just because the civ likes Spark.

### `FarDriveFamily`
No-op until a second Far archetype exists. Documented hook: when Phase 6+ adds `FusionTorch` or similar, gate selection by `civ.FarDriveFamily`. Today's only Far archetype (`RBCABeamCore`) ignores the field.

### `TechTier`
Already wired (Far slot gating). Becomes meaningful immediately because LLM-generated civs span tiers 1–5.

### `DesignPhilosophy`
Not directly consumed by ship gen — it's a narrative anchor that *shaped* the LLM's tech-profile picks at civ-gen time. It's visible in the civ JSON for UI rendering but the ship factory doesn't read it. (Considered using it to bias archetype rarity via keyword matching on terms like "redundant", "exotic", "biological" — rejected as fragile and duplicating what `RiskTolerance` and `ThrustVsIspPreference` already capture.)

---

## 6. Out of Scope (Explicit)

- **Manufacturer-from-civ.** Civs still pull from the shared manufacturer pool. Per-civ manufacturer rosters land when manufacturers themselves become a generated artifact (later phase). The `PickManufacturerForCivilization` provenance bias from Phase 4 keeps working.
- **Cross-civ ship capture / transfer.** `ships.civ_id` exists for it but no mechanic reads or sets it differently from `players.civ_id`.
- **Civ pregeneration / catalog.** Every player still triggers a fresh `GenerateCivilization` on their first ship roll. Batch-pregen lands when we have UI for civ selection.
- **Civ regeneration.** No "reroll civ" endpoint. A player's civ is permanent for their player record.
- **Civ display in UI.** Backend exposes the civ; client wiring is its own scope.
- **Civ-civ relationships.** Diplomacy, trade, war — none of it. Civs are isolated.
- **Replacing `GenericCivilization`.** Still the fallback when `civ == nil` is passed (e.g. the no-civ path in `GenerateRandomShip`).
- **Auth on the civ persistence path.** Same admin-only stance as the rest of the API surface in this iteration.
- **Migration of existing ships' loadouts** to be civ-aware. Old loadouts stay valid as-is; only new rolls use the civ.

---

## 7. Phase 5.1 Validation

End-of-phase acceptance:

- Migration adds `civilizations` table and `civ_id` columns on `players` and `ships` without breaking existing rows.
- `factory.SaveCivilization` round-trips through `LoadCivilization` with all `TechProfile` fields intact (including `RiskTolerance`, `ThrustVsIspPreference`, `DesignPhilosophy`).
- `POST /api/ships/generate?player_id=X` on a player with `civ_id == NULL`:
  - generates and persists a civ (one LLM pipeline run);
  - assigns `players.civ_id`;
  - generates a ship using that civ;
  - persists `ships.civ_id == players.civ_id`.
- Second call with the same `player_id`: civ is loaded (no LLM call), ship rolls fresh against that civ.
- Repeated calls with the same seed *and* same civ produce bit-identical loadouts (determinism preserved per civ).
- Different civs at the same seed produce visibly different loadouts (mixture preferences and archetype weights diverge).
- A high-`RiskTolerance` civ rolls exotic archetypes (RDE, SABRE) noticeably more often than a low-`RiskTolerance` civ over 100 seeds.
- A high-`ThrustVsIspPreference` civ rolls high-Isp archetypes (SCTA) more often than low-preference civs.
- The no-civ path (`GenerateRandomShip(seed, nil)`) still produces valid ships that pass the existing sweep test.
- `FactoryVersion` bumps to `phase5_1-v1`.

---

## 8. Open Questions

- **`civstore` package placement.** Inside `factory/` (close to the type) or `internal/civdb/` (close to the DB)? Current lean: `factory/`. If we add LoadShip/SaveShip later we can revisit consolidation.
- **First-call latency on `POST /api/ships/generate`.** First call now incurs ~3–5s of LLM. Acceptable for admin/test; if it hurts iteration we can add a `?skip_civ=true` debug param. Defer the decision.
- **`ThrustIspBias` values per archetype.** Hand-authored above based on each archetype's premise. Worth a sanity-check pass once the dial is live — easy to retune since it's a single scalar per archetype.
- **Bias formula tuning.** All the multipliers (3.0 for preferred mixtures, 1.5 for preferred cooling, 0.8 for ThrustVsIsp influence) are first-guesses. Plan to retune after observing 50+ generations across civs.
- **Should `GenericCivilization` get explicit `RiskTolerance = 0.5` / `ThrustVsIspPreference = 0.0`?** Currently zero-valued. Zero is fine semantically (balanced). Document in commit, no code change.
- **Should `DesignPhilosophy` ever bite ship gen?** Considered keyword-matching but rejected. Open question whether to revisit if other dials don't produce enough texture.
