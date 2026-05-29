# Phase 6 Plan — Vertical Slice: Walk and Travel
*2026-05-28*

## Goal
Stand up the thinnest possible end-to-end game loop that exercises the architecture of the redesigned `∃`: a hardcoded civ, a hardcoded ship, a hardcoded 5-node sector, three views (orbit, cockpit, console), and the ability to travel between nodes from the console map. The slice is fully in-memory, has no persistence, no auth, no LLM, no time mechanic, and no game-systems mechanics (no fuel, no hull damage, no needs). Travel is instant — click a node on the map, the scene cuts to the new node.

The point is to prove the data path from the Go backend to the Three.js client, the three-view architecture, the sector-graph data model, and the travel mechanic *in their final shape, at minimum fidelity.* Everything else — needs, blueprints, signals, sensors, probes, time, persistence, LLM, captain's log, death — slots in on top of this skeleton in subsequent phases.

---

## 1. The Game-State Shape

A run is in-memory only. A single global `*GameState` lives in the server process and is the source of truth. There is no `runs` table; closing the server drops the run on the floor (you start fresh next time).

The state has three parts:

- **Civ** — the player's civilization. Hardcoded one civ for Phase 6. Reuses the existing `Civilization` struct (no new fields).
- **Ship** — the player's ship. Hardcoded one ship for Phase 6. Reuses the existing ship/assembly types where convenient; details below.
- **World** — a single hardcoded `Sector` containing five `Node`s with edges between them. The world also includes a `CurrentNodeID` field — that's the only mutable bit of state in Phase 6.

The whole state serializes to JSON for the frontend. The frontend doesn't refetch piecemeal — it gets the entire state from `GET /api/state` after each action.

---

## 2. The Hardcoded Civ + Ship

### Civ
One civ, defined in `server/internal/game/civ_hardcoded.go`:

- `Name`: "The Hailing"  (or any short evocative name — pick during implementation)
- `Description`: 1–2 paragraphs of hand-written prose. Establishes vibe; no functional impact in Phase 6.
- `Flavor`: one-liner
- `TechTier`: 2
- `TechProfile`: filled in plausibly with values from existing enums. Doesn't drive anything in Phase 6 but populates the struct so the frontend can render it.
- `AgeYears`: arbitrary plausible value (~50,000)
- `HomeworldDescription`: short prose

No `Planet` is generated for the civ in Phase 6 (we already cut the `HomeworldPlanetID` link from the slice scope; that wiring lands in a later phase).

### Ship
One ship, defined in `server/internal/game/ship_hardcoded.go`:

- A `Ship` struct or a `ShipSummary` shape that includes: a `Name`, an `Aesthetic` (single string field — "industrial slate / orange accents"), and a flat list of named systems (engine, navigation, sensor) with no functional fields yet. Health is 100% across the board (irrelevant in Phase 6 — there's no damage).
- The hardcoded value is constructed once at server startup.

We do **not** rebuild this via `factory/assembly.go` — that path is overkill for one hardcoded ship. We may revisit when Phase 6 closes and ship gen becomes real.

---

## 3. The Hardcoded Sector

New file `server/internal/game/sector_hardcoded.go`. Defines one sector with five nodes.

### Sector
- `ID`: stable UUID string
- `Name`: "Kestrel-7" (or anything)
- `Nodes`: []Node, length 5
- `Edges`: []Edge

### Node
- `ID`: stable string per node (`node-a` ... `node-e` works)
- `Name`: a short readable name per node ("Procyon", "Vega", etc.)
- `Position`: `{X, Y, Z}` in arbitrary units. Used by the console map (and reused by the 3D scene when we want, but only the console map renders the graph). For Phase 6, Z = 0 — the sector is planar.
- `StarType`: short string ("G-type", "M-dwarf", "K-type", etc.)
- `PlanetSeed`: int64 — seeds the existing planet shader. Each node renders a different-looking planet on arrival.
- (Optional, may skip in Phase 6) `PlanetType`: one of the existing `PlanetType` enum values, mostly for future use.

No moons, no atmospheres, no resources. The node is just *"this is where the ship is, with a star and a planet to look at."*

### Edge
- `From`: NodeID
- `To`: NodeID
- Undirected (an edge `A-B` is traversable both ways). No cost field yet — travel is free.

### Layout
Five nodes arranged so the graph is connected but interesting. One reasonable layout:

```
        [Vega]
         / \
        /   \
   [Procyon]---[Altair]    (player starts at Procyon)
        \   /
         \ /
       [Kepler-7]
          |
        [Wolf-359]
```

Edges: Procyon–Vega, Procyon–Altair, Procyon–Kepler-7, Vega–Altair, Kepler-7–Altair, Kepler-7–Wolf-359. Exact layout is implementation detail — the only constraints are (a) connected, (b) at least one node is two hops from start so we exercise multi-step travel.

### Current node
A `CurrentNodeID` field on `GameState`. Initialized to Procyon. The only mutable field in Phase 6.

---

## 4. The Backend Handlers

Two endpoints. Both replace the existing `/api/player`-shaped surface.

### `GET /api/state`
Returns the full game state as JSON:

```json
{
  "civ": { ... },
  "ship": { ... },
  "sector": { "id": "...", "name": "...", "nodes": [...], "edges": [...] },
  "current_node_id": "node-a"
}
```

No query parameters. No auth.

### `POST /api/travel`
Body:
```json
{ "target_node_id": "node-c" }
```

Validates:
- `target_node_id` is a real node in the current sector.
- There exists an edge between `CurrentNodeID` and `target_node_id`.

On success, updates `CurrentNodeID` and returns the full updated state (same shape as `GET /api/state`).

On failure (no such node, no such edge), returns 400 with a short error JSON.

### What gets removed
The existing `/api/player` handler goes away in Phase 6. The existing `/api/ships/generate` admin handler stays — it's harmless and useful for poking the ship pipeline.

### Package placement
A new `server/internal/game/` package holds: `state.go` (the `GameState` struct + a global accessor or singleton), `civ_hardcoded.go`, `ship_hardcoded.go`, `sector_hardcoded.go`. Handlers go in `server/internal/handlers/state.go` and `travel.go`.

---

## 5. Frontend Refactor — Scene Per Node

Today's `scene.js` generates a planet from a seed passed in once at startup. We change it to *render the current node's planet* and to *be re-rendered when the current node changes*.

### Changes
- `main.js` fetches `/api/state` on load instead of `/api/player`.
- `createScene(canvas, state, opts)` takes the full game state. From it, it pulls `state.sector.nodes[currentNode].planet_seed` and `state.sector.nodes[currentNode].star_type` and feeds them to the existing planet + light setup.
- The ship is built from `state.ship` (right now ignored — Phase 6 keeps `buildSpacecraft(rng)` deterministic; we may seed it from a ship-derived value, but no per-system geometry yet).
- The scene exposes a `rebuild(state)` method that tears down and re-instantiates the planet (the ship and starfield can be kept across rebuilds; only the planet changes by node).

### Travel triggers rebuild
After a successful `POST /api/travel`, the client receives the new state, calls `scene.rebuild(state)`, and the user sees the new node's planet. Hard cut — no fade.

### What we don't change in Phase 6
- The starfield stays a single background; it doesn't redraw per-node.
- The orbit parameters of the ship around the planet stay the same logic (seeded from node seed for variety).
- The cockpit FBX stays where it is — no per-civ ship interiors yet.

---

## 6. The Console View (Orthographic Mini-Map)

The existing `PANEL` view is a stub. Phase 6 turns it into the **console view** — a 2D-feeling top-down view of the sector graph, rendered as a Three.js orthographic scene.

### Implementation shape
A separate `THREE.Scene` and `THREE.OrthographicCamera`, instantiated alongside the main scene, sharing the renderer. When the player is in console view, we render this scene instead of the main one.

### What's in the scene
- One mesh per node. A simple circle (CircleGeometry or a Sprite) at the node's `(X, Y)` position.
- One line per edge (`THREE.Line` with two-point geometry).
- The current node is highlighted (different color, slightly larger, or pulsing).
- Reachable nodes (one edge away from current) are rendered with a "selectable" style.
- Unreachable nodes (further away) are rendered dimmer.

### Labels
Node names are rendered as `THREE.Sprite`s using a small text-to-canvas-to-texture helper. (Phase 6 can also skip labels entirely and just show node IDs — we'll see how it reads.)

### Click handling
A `THREE.Raycaster` listens for clicks on the canvas while in console view. If the click hits a node mesh and that node is reachable (edge from current), the client fires `POST /api/travel` with that node's ID. On success, the main scene rebuilds and the player auto-switches back to orbit view (or stays in console — see Open Questions).

### Styling
Same monochrome / vintage-NASA feel as the cockpit. White or pale-green markers on a black background. No gradients, no glow. The console is information, not art.

---

## 7. Travel UX

**Hard cut.** Click → API call → on response, main scene rebuilds with the new node's data and the player is dropped into orbit view at the new node. No fade, no warp, no loading state beyond a brief network round-trip (which is local for Phase 6).

A short "ARRIVING AT NODE-NAME" toast in the HUD is fine but not required.

If the travel API call fails for any reason, the HUD shows a one-line error and nothing changes.

---

## 8. What FactoryVersion Becomes

`FactoryVersion` stays at `phase5-v1` for now. Phase 6 doesn't change civ or ship shape — it adds an orthogonal `game` package that wraps existing types. No factory bump.

---

## 9. Out of Scope (Explicit)

- **Time / tick simulation.** Travel is instant. No offline-time advancement, no scheduled events.
- **Persistence.** No DB writes. Server restart = new run.
- **Ship needs.** No fuel, no hull, no life support. No meters in the HUD.
- **Multiple sectors.** One sector only. No sector boundary, no nav-tier gating.
- **LLM-driven anything.** No civ generation, no description generation at runtime. The single civ is hardcoded.
- **Blueprints, the captain's log, probes, sensors, signals, beacons, wrecks.** Not in Phase 6.
- **Death and restart.** You can travel forever. No death conditions.
- **Auth / multi-user.** One process serves one global state.
- **The `/api/ships/generate` admin endpoint changes.** Leave it as-is; not part of Phase 6's surface.
- **Refactoring `factory/flight/` or `factory/mixtures.go`.** Out of scope; touched only if compilation breaks.
- **Sector-graph procgen.** Hardcoded sector only. Procgen is its own phase.
- **Per-node lighting / star color variation in the 3D scene.** Star type is in the data but the visual uses the existing single sun light. Color variation per star is a polish pass for later.

---

## 10. Phase 6 Validation

End-of-phase acceptance:

- `GET /api/state` returns a valid JSON payload with civ, ship, sector (5 nodes + edges), and `current_node_id == "node-procyon"` (or whichever node we choose as start).
- `POST /api/travel` with a valid neighboring node updates state and returns the updated payload.
- `POST /api/travel` with an unreachable or unknown node returns 400.
- On client load, the main scene renders the current node's planet (seeded from `planet_seed`).
- Pressing `1` shows orbit view, `2` shows cockpit view, `3` shows console view. Keyboard `V` cycles through them.
- The console view (key `3`) shows an orthographic top-down map: five node markers, edges between them, the current node highlighted, reachable nodes selectable.
- Clicking a reachable node on the map triggers travel; the main scene rebuilds with the new node's planet on arrival.
- Clicking an unreachable node either does nothing or shows a "no route" hint (implementer's call).
- `client/src/main.js` no longer references `/api/player`.
- The HUD continues to show some basic info (player ID can be dropped; current node name should be visible).

---

## 11. Open Questions

- **Auto-switch back to orbit on travel arrival?** When the player clicks a node in the console and travel succeeds, do we (a) auto-switch them to orbit so they immediately see the new planet, or (b) leave them on the console so they can plan the next hop? Recommendation: (a) — the arrival is the reward, show it.
- **Edge selectability vs. node selectability.** Do we click *the destination node* (recommended), or click *the edge*? Edge-click is more "I am choosing this route" but UX-fussier. Recommendation: node-click.
- **HUD what to show now.** The HUD currently shows player ID, system seed, orbit info, view name. Player ID can go. Suggest: current node name, current sector name, view name. Drop the orbit numbers — they're not load-bearing.
- **Starfield re-render per node.** Recommendation: no — keep the single starfield. The starfield is "the universe," not "the sky from this planet." Revisit in a polish phase.
- **Node-name labels in the console scene.** Sprites are fine but cost. Recommendation: implement them — the map is unreadable without labels. If labeling is fiddly, fall back to HTML labels positioned in screen space over the canvas.
- **One civ vs. picking civ at run start.** Recommendation: one civ for Phase 6. The "pick from a curated set" mechanic lands when curation becomes real (when LLM-generated civs are baked).
- **What does the cockpit view show during console?** Recommendation: nothing — the cockpit model only renders in cockpit view; the orthographic scene replaces the main scene entirely when in console view.
- **Where the game state singleton lives.** Two options: (a) a `var Current *GameState` in the `game` package, mutex-protected; (b) the state held by the HTTP server and passed into handlers via context. Recommendation: (a) — simpler, fine for single-process single-player.

---

## 12. Phase 6 → Phase 7 Glide Path

The natural next phase is **Player-Authored Civilization** — the game's central differentiator. Phase 7 lands the guided-intake conversation, the LLM translation from player prose into the `Civilization` struct, the civ-driven ship roll, and the distance-from-home calculation (your civ's tier determines your starting offset from the goal). Phase 7 still runs in-memory and without time mechanics — it just replaces Phase 6's hardcoded civ with an authored one. By the end of Phase 7, the game's unique mechanic is provably alive at minimum fidelity.

Subsequent phases, in order:

- **Phase 8 — Time + Persistence.** Postgres-back the game state; add the daily-tick offline advancement on a log-capped scale; make travel take real time. Replaces the in-memory singleton with a `runs` table and a tick worker.
- **Phase 9 — Needs.** Fuel, hull, life support. The first decay/consume loop. Travel and time become *costly,* not just *slow.*
- **Phase 10 — Blueprints.** The recipe catalog, the discovery rule, the deck UI, the "remembered" carryover across runs.

The order is deliberate: civ-authoring before time because the civ is the front door and must feel right before any system layers on; time before needs because needs without time-advancement are meaningless; blueprints after needs because blueprints' first role is "fix the things needs are breaking."
