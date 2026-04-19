# Archetypes — ELI5

*Draft. Eventually this lives in-game as a player-facing codex. For now it's a reference for the author.*

## What's an archetype?

An **archetype** is a recipe that tells the factory how to make one kind of thing. It doesn't describe a specific object — it describes a *family* of possible objects, with ranges and rules.

Think of it like a blueprint for "sedan" rather than a blueprint for "this specific 2019 Toyota Camry." The sedan blueprint says things like "engine displacement is somewhere between 1.5 and 3.5 liters" and "wheelbase is 2.7 to 3.0 meters." Every car rolled off the line is different, but every car is recognizably a sedan.

When the factory generates a ship, it picks an archetype for each part, then uses that archetype's ranges + a random seed to roll one specific instance.

## Category vs. archetype — a confusing bit

The code has two layers that are easy to mix up:

- **Category**: a *kind* of system. "Liquid-chemical engine" is one category. "Ion thruster" will be another. Categories are defined by a shared Go struct and a shared generator — they describe what fields exist and how to roll values into them.
- **Archetype**: a *specific template within* a category. `RCSLiquidChemical` is one archetype of the liquid-chemical engine category. All archetypes in the same category use the same generator; they only differ in the number ranges they feed it.

So when we say "this engine came from the `RCSLiquidChemical` archetype," we mean "it was rolled from the liquid-chemical category, using the specific ranges for small reaction-control thrusters." Another engine from `OMSLiquidChemical` would also be a liquid-chemical engine — but its ranges would produce something much beefier.

## What's in an archetype?

Two kinds of things:

1. **Ranges for scalar numbers** — chamber pressure between 5 and 25 bar, dry mass between 1 and 50 kg, thrust between 50 and 1000 N.
2. **Allowed lists for enums** — this archetype can use `MMH/NTO` or `Hydrazine` as propellant; it can use `Ablative`, `Radiative`, or `Film` cooling.

When the factory rolls an instance, it doesn't sample every field independently. It walks a dependency graph (a DAG): chamber pressure rolls first, and everything downstream (Isp, thrust, cooling options, thermal behavior) is conditioned on that roll. A high-pressure roll tends to produce a punchier engine with tighter restart ceilings and fewer allowed cooling methods. A low-pressure roll produces something gentler and more forgiving.

## Archetypes we have

### `RCSLiquidChemical` — Reaction Control System thrusters (Short slot)

The only archetype shipping in Phase 3. Small, numerous, on-off thrusters used for fine maneuvering — docking, sample collection, rotating the ship. Think the little puffers you see on the Apollo lunar module or the ISS.

**Shape:** low chamber pressure (5–25 bar), low thrust (50–1000 N), tiny dry mass (1–50 kg), unlimited restarts (they fire thousands of times over a mission), no gimbal (RCS pods are fixed; you steer by firing opposing pairs), on-off throttle (no deep-throttle modulation), short burn duration (1s to 5min). Propellants are simple and reliable — `MMH/NTO` (hypergolic — ignites on contact, no spark needed) or `Hydrazine` (monopropellant — catalyst bed decomposes it, no oxidizer plumbing).

**Why it's the first archetype to ship:** smallest end-to-end thing that exercises every feature of the generator — propellant derivation, cooling filtering, ablator tracking, TechTier-narrowed health — without needing complex ship-level sizing logic.

## Archetypes we'll add later (sketched in Plan §2)

### `OMSLiquidChemical` — Orbital Maneuvering System (Medium slot)

Mid-sized engines for orbital transfers, planetary takeoff/landing. Think the OMS pods on the Space Shuttle, or the main engine on a lunar lander. Much higher thrust than RCS (10–200 kN), heavier (100–2000 kg), does gimbal (you steer the thrust vector to control attitude during a burn), throttleable, runs for minutes to hours. Same liquid-chemical category — same struct, same generator — just different ranges.

### `BoosterLiquidChemical` — first-stage boosters (Medium slot)

The giant engines that haul a ship off a planet. Very high thrust (hundreds of kN to MN), high chamber pressure (100–300 bar → access to regenerative cooling), heavy, limited restart count (sometimes just one — light them, burn them, stage them). Shares the liquid-chemical generator but the ranges push into a completely different physics regime.

### `HypergolicStorable` — "storable" bipropellants

An archetype that restricts `AllowedMixtureIDs` to hypergolic, non-cryogenic mixtures. These engines can sit on a pad (or in deep space) for years without active cooling, but the propellants are toxic and corrosive. Good for long-duration missions where reliability matters more than performance. Same liquid-chemical category — the restriction is just in the allowed-mixture list.

### `CryogenicUpperStage` — high-Isp vacuum engines

The opposite trade: restrict `AllowedMixtureIDs` to cryogenic mixtures (`LOX/LH2`, `LOX/CH4`). Enormously higher Isp, but propellants boil off constantly, restarts are limited, every hour in space costs fuel. Again, same liquid-chemical category — different range emphasis and different mixture allow-list.

## Other categories (not built yet — Phase 4+)

Liquid chemical is just the first of many propulsion categories. The Plan sketches:

- **Ion drive** (Short slot or Medium slot depending on archetype) — electric propulsion, tiny thrust, enormous Isp, needs huge power budget. Runs for months continuously.
- **Solid rocket** (Medium slot) — fire once, can't throttle, can't shut down. Cheap and simple.
- **Cold gas** (Short slot) — just a compressed gas through a nozzle. Terrible Isp, but reliable and simple — good for tiny satellites.
- **Fusion torch** (Far slot) — hypothetical; Plan §2 reserves this for interstellar transit.
- **NLS variants** (Far slot) — "near-light-speed" drives: Alcubierre bubbles, antimatter torches, laser-pushed lightsails. Deliberately soft sci-fi so players don't have to wait real-world decades for interstellar transit.

Each of these will be its own *category* — its own struct type, its own generator, its own set of archetypes. A ship's three flight slots will pull across categories: a ship might have cold-gas RCS in the Short slot, a solid booster in the Medium slot, and a fusion torch in the Far slot — or any other combination the civilizations have access to.

## How quality enters the picture

Archetypes don't have a "good/bad" knob. Quality is inherited through the chain `Manufacturer → Civilization → TechTier`. When the factory rolls an engine:

1. Pick a primary civilization for the ship (Phase 3: always `GenericCivilization`; Phase 4: varies).
2. Pick a manufacturer belonging to that civilization.
3. Use that manufacturer's civilization's `TechTier` (1–5) to narrow the archetype's `HealthInitRange` — higher tier means the engine rolls closer to the top of the health band.

So two engines rolled from the same `RCSLiquidChemical` archetype can feel very different: one from a Tier-5 civilization's top-shelf manufacturer will be near-perfect; one from a Tier-1 civilization's dubious workshop will be visibly rougher. The archetype didn't change — the provenance did.

## When to add a new archetype vs. extend an existing one

**Extend an existing archetype** (widen ranges, add an enum value) when the change is quantitative: "I want RCS thrusters to cover a bigger dry-mass range."

**Add a new archetype** when the *shape* of the engine changes: different restart semantics, different cooling options unlocked, different propellant allow-list. `BoosterLiquidChemical` and `RCSLiquidChemical` are both liquid-chemical engines, but their ranges are so different that a single archetype covering both would feel incoherent — you'd roll a 1kg booster or a 1-ton RCS thruster.

**Escalate to a new category** when the *physics* changes: ion drives don't belong in the liquid-chemical struct because half the liquid-chemical fields (chamber pressure, propellant config, ablator mass) don't mean anything for an ion drive.
