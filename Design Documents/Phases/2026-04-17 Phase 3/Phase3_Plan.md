# Phase 3 Plan — The Factory Service
*2026-04-17*

## Goal
Stand up a dedicated Python **Factory Service** that owns the procedural assembly of all spacecraft (and, by the end of the phase, probes). The Go backend stops generating ships and instead delegates to the Factory over HTTP. Ship generation grows from "pick one row per type at random" (Phase 2) into a constraint-driven assembly that produces internally-consistent vehicles whose visuals reflect their systems.

This is the major lift of Phase 3 — a new service, a new language in the stack, a new deployment surface, and a richer data model. The user-facing payoff is that every player gets a ship that *makes sense* (its power budget closes, its thrusters match its mass, its radiators match its thermal load) and *looks* like the systems it carries.

---

## 1. Architecture

```
┌────────┐    GET /api/player        ┌────────┐    POST /ships     ┌──────────┐
│ Client │ ────────────────────────► │   Go   │ ─────────────────► │ Factory  │
│  (TS)  │ ◄──────────────────────── │ (API)  │ ◄───────────────── │ (Python) │
└────────┘     player + ship         └────────┘   ship config      └──────────┘
                                          │                              │
                                          ▼                              ▼
                                     ┌─────────┐                  catalog.json
                                     │ Postgres│                  (read-only)
                                     └─────────┘
```

- **Client** unchanged in shape — still hits `GET /api/player`, still receives a player object that now embeds a richer ship config.
- **Go backend** owns persistence (`players`, `ships`), session, and routing. On player creation it calls the Factory; on returning visits it loads the stored ship from Postgres.
- **Factory service** is *stateless*. It owns the catalog data file and the assembly logic. Given a seed (and optionally constraints), it returns a fully-specified ship configuration. It does not read or write the database.

### Why stateless
Stateless keeps deploy/ops simple: no migrations, no DB credentials in the Python service, easy horizontal scaling, trivial local dev. The catalog ships as a JSON file in the Python image; updating it is a release, not a migration. If the factory ever needs persistent state (e.g. caching expensive solver runs), revisit then.

---

## 2. Factory Service Spec

### Stack
- **FastAPI** — async HTTP, type hints via Pydantic, OpenAPI for free.
- **Pydantic** for request/response models and catalog validation on startup.
- **NumPy** where the solver benefits (mass/power optimization, scoring).
- **uvicorn** in container, **pytest** for tests.

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/ships` | Generate a ship from a seed + optional constraints |
| `POST` | `/probes` | Generate a probe (added late in the phase) |
| `GET`  | `/catalog` | Return the loaded systems catalog (debug / admin) |
| `GET`  | `/healthz` | Liveness — returns version + catalog hash |

### `POST /ships` request

```json
{
  "seed": 1234567890,
  "archetype": "scout" | "freighter" | "explorer" | null,
  "constraints": {
    "max_mass_kg": 50000,
    "min_delta_v_ms": 8000
  }
}
```

`archetype` and `constraints` are both optional. If omitted, the factory picks an archetype from the seed and uses default bounds.

### `POST /ships` response

```json
{
  "ship_id_hint": "uuid-suggested-by-factory-or-null",
  "archetype": "scout",
  "seed": 1234567890,
  "systems": [
    { "id": "...", "type": "flight",       "name": "Ion Thruster Mk II", "attributes": {...} },
    { "id": "...", "type": "power",        "name": "RTG-7",              "attributes": {...} },
    ...
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
}
```

The Go backend persists `systems` and `derived`. The `visual` block is what the frontend reads to assemble the procedural ship mesh in Phase 3+ (replaces the placeholder geometry of Phase 2).

### Service-to-service auth
Shared bearer token in `FACTORY_TOKEN` env var, set in `docker-compose.yml` for both services. Cheap, sufficient for an internal service. Revisit if the factory ever becomes externally reachable.

---

## 3. Catalog Data Model

The catalog ships as `catalog.json` (or YAML — TBD) inside the Python image. It is loaded and validated on startup; a malformed catalog is a hard fail. Each entry is a *variant* of a system.

```json
{
  "version": "2026-04-17",
  "systems": [
    {
      "id": "flight.ion.mk2",
      "type": "flight",
      "name": "Ion Thruster Mk II",
      "tags": ["efficient", "low-thrust"],
      "attributes": {
        "thrust_n": 0.25,
        "isp_s": 4200,
        "power_draw_w": 2400,
        "mass_kg": 180,
        "thermal_load_w": 600
      },
      "visual": {
        "thruster_geom": "ion_quad",
        "nozzle_count": 4
      },
      "compatible_with": { "power": ["rtg.*", "fission.*"] }
    }
  ]
}
```

### Types (Phase 2 catalog → unchanged)
`flight`, `power`, `sensor`, `structural`, `thermal`, `life_support`, `propellant`, `engineering`, `probe`

### What's new in Phase 3
- `tags` — for archetype scoring ("scout" prefers `low-mass`, `efficient`).
- `attributes` — typed numeric fields used by the solver (mass, power, thermal, etc.).
- `compatible_with` — soft compatibility hints expressed as type → list of glob patterns over `id`. Used to prune the search space.
- `visual` — geometry hints the frontend uses to assemble the mesh.

### Ownership
The catalog is a **data file in the Factory repo/image**, not a database table. This kills a class of cross-service issues (who migrates it? when?) and makes it trivial to review in PRs. The Go service has no concept of the catalog beyond what it sees on a `/ships` response.

---

## 4. Ship Generation Algorithm

Phase 2's algorithm was "pick one row per type at random." Phase 3 introduces a small constraint solver. Roughly:

1. **Seed** the factory's RNG (`numpy.random.default_rng(seed)`) so generation is reproducible.
2. **Pick an archetype** if not specified. Archetype defines:
   - Required types (every ship has flight + power + structural + life_support; scouts also get sensor; freighters get extra propellant; etc.)
   - Tag preferences (weighted scoring)
   - Constraint defaults (mass cap, min Δv, etc.)
3. **Candidate generation**: for each required type, take the catalog rows of that type, filter by `compatible_with` against already-chosen systems, score by archetype tag preference + RNG jitter.
4. **Solve**: greedy first pass, then if derived stats violate constraints (power doesn't close, mass over cap, thermal under-served), backtrack and try the next-best variant. Cap iterations at a small number — if no solution found, relax constraints and accept.
5. **Compute derived stats**: dry mass, peak power, thermal load, Δv from rocket equation.
6. **Compute visual block**: hull form chosen from archetype, thruster geometry from the chosen flight system, panel area from the power system's panel-needs attribute, etc.
7. **Return**.

The solver is intentionally *simple* (greedy + small backtrack). We're not going for optimal — we're going for *consistent*. If/when this is too crude, we can reach for `python-constraint`, `mip`, or write a small simulated annealer.

---

## 5. Go Backend Changes

### New responsibilities
- Hold a `FactoryClient` (HTTP client with the bearer token, base URL from env).
- On player creation: call `factory.GenerateShip(seed, nil)`, persist the response.
- On player load: read ship from DB, return embedded.

### Removed
- Any random-system-pick logic that may have been added in Phase 2 stays — and gets deleted in Phase 3.

### `GET /api/player` response shape (extended)
```json
{
  "id": "...",
  "seed": 1234567890,
  "ship": {
    "id": "...",
    "archetype": "scout",
    "systems": [...],
    "derived": {...},
    "visual": {...}
  }
}
```

---

## 6. DB Changes

| Change | Notes |
|---|---|
| **Drop** `system_catalog` table from Phase 2 | Catalog now lives in the Factory's image, not the DB. Migration removes the table. |
| **Extend** `ships` table | Add `archetype TEXT`, `derived JSONB`, `visual JSONB`. Keep `systems JSONB` (now stores the full system objects, not just IDs — denormalized snapshot at generation time). |
| **Add** `ships.factory_version TEXT` | Records which catalog version produced the ship. Lets us reason about ships generated against old catalogs after updates. |

Snapshotting the full system objects (rather than referencing catalog IDs) means a catalog change never silently mutates an existing player's ship. The catalog is a generator input, not a runtime lookup.

---

## 7. Deployment & Local Dev

### Docker Compose additions
A new `factory` service:
- Image built from `factory/Dockerfile` (Python 3.12 slim + uvicorn).
- Internal port 8000, not exposed to host in prod (Go talks to it over the compose network at `http://factory:8000`).
- Exposed locally on `8001` for direct curl/debug.
- Env: `FACTORY_TOKEN` (shared with `api`), `LOG_LEVEL`.
- Healthcheck on `/healthz`.

### CI
- Add a `factory` job to the existing CI: `pip install -r requirements.txt`, `pytest`, lint (`ruff`).
- Build + push the factory image alongside the API image.

### Local dev
- `docker-compose up` brings up `db`, `api`, `factory`, `client`.
- `factory/` has `make dev` for `uvicorn --reload` outside docker when iterating on the catalog.

---

## 8. Frontend Changes

The frontend **reads** the new ship config but most of the visual changes are scaffolding for later phases. Specifically in Phase 3:

- Replace the hardcoded `buildSpacecraft(rng)` in `client/src/scene.js` with a `buildSpacecraft(shipVisual)` that consumes the `visual` block from the API response.
- Hull form, thruster geometry, panel area, radiator area, and accent hue all become data-driven.
- Phase 2's placeholder geometry is retained as the *fallback* when fields are missing.
- HUD gets a small "SHIP: <archetype>" line so the player can see which archetype they rolled.

No cockpit changes in this phase — that lives in Phase 4 (system-specific cockpit instruments).

---

## 9. Open Questions

- **Catalog format**: JSON or YAML? YAML is friendlier to hand-edit and supports comments; JSON is universal and matches what the API returns. *Lean: YAML on disk, validated into the same Pydantic models the API uses.*
- **Archetype list**: How many at launch? *Lean: 3 (scout, freighter, explorer) to keep variety meaningful but the catalog small.*
- **Determinism**: Is the assembly fully deterministic from seed alone, or seed + catalog version? *Lean: seed + catalog version. Persist `factory_version` so we can reproduce a ship later.*
- **Schema sharing**: Hand-mirror Pydantic ↔ Go structs, or generate from a shared schema? *Lean: hand-mirror for now (small surface, two endpoints). Revisit when it bites.*
- **Probe factory in scope?** Listed above as a stretch goal. If it slips to Phase 4, the architecture doesn't change — same service, new endpoint.

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

- A new player loads the app, hits `GET /api/player`, and the Go backend (on first visit) calls the Factory over the compose network.
- The returned ship has internally-consistent derived stats: power closes, mass under cap, thermal load served.
- The frontend renders a placeholder ship whose **shape** changes visibly with the rolled archetype and systems (different thruster, different panel size).
- The factory has at least 6 catalog entries per system type and 3 archetypes.
- CI builds and tests both services on every PR.
- Local `docker-compose up` brings the whole stack up cleanly.
