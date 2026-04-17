# Phase 2 Plan — Camera & Ship Creation
*2026-04-17*

## Goal
At the end of Phase 2, a user opens the app, receives a randomly generated ship, and can observe it from three distinct camera views.

---

## 1. Camera System

Three views, switchable with `1` / `2` / `3` (or `V` to cycle):

| Key | View | Description |
|---|---|---|
| `1` | **Orbit** | OrbitControls targeting `ship.position` (not planet). Player is the center. Planet is background. |
| `2` | **Cockpit** | Camera parented to the ship group, positioned at cockpit offset, looking forward out the window. Minimalist 3D interior (Blender assets). |
| `3` | **Control Panel** | Camera looks at instrument panel. DOM overlay shows ship system readouts. Placeholder for the OS terminal (Phase 3). |

---

## 2. Ship Creation

### Systems Catalog (static DB table)
A fixed list of all known system types and their variants — not generated, just seeded from a data file on first run. Each row is a system variant with a type, name, and attributes.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID | |
| `type` | ENUM | `flight`, `power`, `sensor`, `structural`, `thermal`, `life_support`, `propellant`, `engineering`, `probe` |
| `name` | TEXT | e.g. `Ion Thruster Mk II` |
| `attributes` | JSONB | type-specific stats (efficiency, power draw, mass, etc.) |

### Ship Generation
On player creation, the server picks one system of each type at random (seeded from the player seed). Ship configuration is stored in a new `ships` table linked to the player.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID | |
| `player_id` | UUID FK | |
| `systems` | JSONB | Array of selected system IDs |
| `created_at` | TIMESTAMPTZ | |

### Visuals
- 3D ship model and cockpit interior: **Blender assets** (user-designed, to be added in Phase 2)
- Placeholder geometry used until assets are ready
- Ship appearance reflects its systems (e.g. large radiators for high thermal load, different thruster geometry for ion vs chemical)

---

## DB Changes
- Add `system_catalog` table (seeded from static data)
- Add `ships` table
- `GET /api/player` response extends to include ship configuration

---

## Out of Scope for Phase 2
- Flight controls / Δv simulation
- Sensor scanning
- Spacecraft OS CLI
- Multi-player traces
