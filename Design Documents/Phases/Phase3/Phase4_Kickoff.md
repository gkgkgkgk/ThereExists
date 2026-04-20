# Phase 4 Kickoff — Things to Remember

Short list of loose ends from Phase 3 and natural next steps. Not a plan — just breadcrumbs.

## Picked up from code TODOs

- **Tighten `ShipLoadout.Flight`** from `map[FlightSlot]any` to `map[FlightSlot]FlightSystem` once `FlightSystem` grows methods (currently an empty interface). See `factory/assembly/ship.go`.
- **Implement `FlightSystem.Tick(dt, throttle)`** — wear, thermal, ablator depletion. Phase 3 panics as a tripwire.
- **Implement `HeatToHullW(throttle)`** — real formula, not the zero stub. Needed before the ship-level thermal bus lands.
- **Medium and Far flight slots** — Phase 3 only registers `Short` (RCSLiquidChemical). Need at least one archetype per remaining slot.
- **Rarity weights** for archetype selection in `GenerateForSlot` (currently uniform).

## Damage / repair sim

- Ship redundancy is modeled: `SystemBase.Count` + `Health []float64` (per-unit).
- Next: migrate `Health []float64` -> `[]UnitState{ health, break_reason }` where `break_reason ∈ {nominal, fuel_line, ignition, pump, ...}`. Different breaks drive different repair flows. Bump `FactoryVersion` when landing.
- Decide how damage enters the sim (combat? wear over time? failure rolls on restart?).

## API / infra

- `/api/ships/generate` is currently debug-only and requires `player_id`. Decide whether Phase 4 wires it into real gameplay or keeps it as a dev tool.
- Swagger UI served at `/swagger/index.html`; regeneration runs automatically via `dev.ps1`.
- Adminer at `localhost:8081` for DB inspection.

## Frontend

- Camera modes (orbit + cockpit) exist on separate branches/commits but are **not integrated** into the current `scene.js`. Pull them in early in Phase 4 so ship-state debug visuals have somewhere to live.

## Schema

- `factory_version` column on `ships` exists; nothing reads it yet. First Phase 4 use: filter out loadouts from incompatible factory versions on load, or trigger a regenerate.
