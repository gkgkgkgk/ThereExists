# $\exists$ (There Exists)
*A quiet game about getting home.*

> **Note on this revision.** This document supersedes `Archive/ThereExists_v1.md`. The original was a hardcore real-time engineering simulation in a shared multiplayer galaxy. This version keeps the aesthetic, the tech stack, the AI-driven procgen, and three of the four core pillars — and replaces the simulation-depth-as-gameplay center of gravity with a chill, single-player, check-in–paced quest game. The sim-flavor design docs (`TE_Flight`, `TE_Probes`, `TE_Sensors`, etc.) are archived rather than deleted; some of their ideas survive in much-simplified form here, and the rest are reference material.

## Vision
You don't pick your civilization. You write it.

Each run begins with a quiet conversation: a console asks you who your people are, what your homeworld looks like, what you value, what you build. You answer in your own words. The universe reads what you wrote and makes it real — your ship, your starter blueprints, your console's voice, the silhouette of your hull. You wake up in deep space in a vessel that came out of *your* civilization's hands.

You have no memory of how you got here. Your ship is broken, your nav charts are blank past a few light-years, and home is a name you remember without a coordinate. You have a handful of blueprints, a few weeks of supplies, and one functional probe.

You check in once a day for five or ten minutes. You scan, you decide one or two things, you close the tab. The world advances while you're away. Some runs you make it home. Most you don't. The captain's log remembers everything you wrote and everything you did.

## Core Pillars

### The Sublime Void
Space is indifferent. Discovery should feel statistically improbable and emotionally heavy. Every new node on the map, every signal interpreted, every derelict opened is a small miracle against a black background.

### Asynchronous Stewardship
The game runs in real-time when you're playing, and on a logarithmic, capped scale when you're not. Three days away or three weeks away returns roughly the same state — you can put it down without guilt, but you cannot stockpile time. The waiting is part of the weight: travel takes hours or days, decisions commit you to outcomes you won't see until tomorrow.

### Math-as-Art
Procedural generation drives what is *possible*; AI-generated lore paints what it *feels like*. Sensor reads come back as point clouds, spectrograms, and terse log lines. The visual language is vintage-NASA / used-future: low-poly, monochrome HUDs, schematic overlays. No hand-drawn art.

### Echoes Across Lives *(replaces "Together but Alone")*
You die. A lot. Each run is a new civ you wrote, a new ship that came from it, a new sector dropped far from anywhere you've been before. But the captain's log persists across all your lives — names you gave, places you reached, blueprints you discovered, *the prose you wrote* for each civilization you brought into being. Very rarely, a new run drifts near the wreck of a past run, and you can recover what that captain carried. The universe is empty of other players by design; instead, it slowly fills with the civilizations you authored.

### Prose is Gameplay-Agnostic *(supporting principle)*
The player writes their civilization, but they do not write their *mechanical advantage.* "We are an advanced spacefaring people" is allowed; "we have the fastest, most unbreakable ships" is not. The game interprets the description and assigns mechanical consequences — including tradeoffs the player did not choose. The contract is: you describe character, the universe decides cost.

### Distance is the Master Difficulty Scalar *(supporting principle)*
Power has a price. A more capable civilization wakes up *farther from home.* A monastic low-tier culture might begin three sectors out; a galaxy-spanning empire might begin twenty. There is no "easy mode" via prose. Every authored civilization receives a fair, hard run — capability buys reach, not advantage.

## The Player Experience

### Run Start — Authoring Your Civilization
Before the run begins, the console runs a short guided intake. The LLM, in a quiet, patient voice, asks one question at a time:

> *What does your homeworld look like? What's the sky?*
>
> *And what do your people value above all?*
>
> *How do they build? What does their work look like?*
>
> *Is there something they fear, or refuse?*

The player answers in their own words, conversationally. The intake might run six or eight turns. The LLM is encouraged to push back gently, ask follow-ups, name things back to the player ("So *The Hailing* are a long-lived people, then?"). The intake is the first place the game's tone shows up.

At the end, the LLM produces a structured civilization: tech tier, preferred mixtures, cooling and ignition methods, far-drive family, starter blueprint deck, ship aesthetic, ship name, and a "console voice" — a short style guide that flavors every system report for the rest of the run. *Reactor stable* becomes *The Heart sings well today,* or *RX-9 nominal,* depending on who you are.

A "roll random" button is always available for players who don't want to author, or who want a fast restart after death. The random path runs the same intake under the hood with model-authored answers.

### A Typical Check-In
You open the app. The console shows you arrived overnight at a node you'd plotted — a yellow dwarf with two rocky bodies and a debris field. The probe you launched two days ago has returned data from the next system over; its spectrograph picked up something interesting. Hull is at 78%, fuel at 41%, life support at 63%. One of your three life-support recyclers threw a warning at 04:17 ship-time.

You decide: investigate the debris field (might be salvage, might be nothing), or skip ahead toward the probe's reading? You glance at the captain's log — the breadcrumb trail toward home is faint here. You pick salvage, dispatch the probe to scout the *next* hop, and acknowledge the recycler warning so it stops flashing. You close the tab.

Tomorrow you'll find out what was in the debris field.

### What the player does, distilled
- **Observe.** Read sensor outputs, schematics, log entries, probe reports.
- **Interpret.** Decide what a signal, a spectrogram, a fragment of alien text actually means.
- **Decide.** Pick one or two actions per session — where to go, what to repair, what to salvage, what to ignore.
- **Wait.** The next session reveals the outcomes.

The four problem-solving flavors — logistics, diagnostic, interpretation, narrative — show up in combination, not isolation. A salvage decision is logistical (fuel cost vs. payoff), diagnostic (what subsystem could use the part), interpretive (what *is* this wreck), and narrative (whose was it).

## The Ship

### Persistence
One ship per run. You don't switch ships mid-run. The ship is procedurally rolled at the start of a run from your civ's tech profile — its size, hull style, system layout, even its UI typography. When you die, the ship is gone.

### Needs
Three meters, all decay over time, all replenished by play:
- **Fuel / Power** — burned during travel and operation. Replenished by stars, salvage, and certain planetary resources.
- **Hull Integrity** — degrades from events (debris, storms, weapons, age). Repaired with blueprints and salvaged parts.
- **Life Support / Supplies** — consumed at a steady rate. Replenished from biocompatible planets and supply caches.

No fourth "morale" or "AI mood" need. The ship is a tool, not a character.

### Blueprints (the meta-currency)
Blueprints are cards. Each card is a small recipe: *repair a fuel injector*, *upgrade scanner range +20%*, *fabricate a probe casing from titanium scrap*, *retrofit an alien drive into your hull*. A run begins with a starter deck of 5–12 blueprints, determined by your starting civ's tech tier and culture.

During a run you can discover *new* blueprints in three ways:
1. **Recovery** — salvage a derelict, recover its captain's deck. (Including rare encounters with your own past runs.)
2. **Discovery** — the game has a hidden table of recipes. When your current ship + tools + on-hand resources satisfy a recipe's preconditions, you can "discover" it during a check-in. The discovery surfaces as a sensor read or a workshop hint.
3. **Trade with extant civs** — if your route crosses a contemporary civilization (rare), some interactions yield blueprint exchanges.

Blueprints discovered in a run get added to the *permanent collection* on the captain's log when the run ends. Future runs see them in a separate "remembered" deck and can re-acquire them under certain conditions.

### The Co-Pilot
There is no AI co-pilot personality. System reports are filtered through a clean, utilitarian console voice. We keep the silence of space.

## The Universe

### Sectors of Nodes
Space is a graph. Each node is a star system (or sometimes a notable isolated object: a derelict relay, a rogue planet, a debris cloud). Edges between nodes have travel costs in time and fuel. Nodes cluster into **sectors**; a sector is a sub-map.

You only see what your nav can resolve. Within your scan range, edges and node summaries are visible. Beyond it, nodes appear as `[??]`. A nav upgrade extends your scan range; a major nav upgrade may unlock the route between sectors.

Home is in a sector you cannot reach at start. You probably cannot even see its sector from your starting one. Finding home is a chain of breadcrumb discoveries that piece together the route.

### Star Systems
Each node, when visited, instantiates as a small star system with one star (or two), 0–4 planets, and zero or more notable features (asteroid belts, derelicts, signals, beacons). Planets use the existing planet pipeline (kept and simplified). System layout is procedural; what's *in* the system is a mix of procgen and LLM-generated lore.

### Civilizations
Civilizations are the texture of the universe. The existing `factory/civgen.go` pipeline gets reshaped into two modes:
- **Your civ.** Authored by the player at run start through a guided conversation. The LLM translates the prose into the full civ struct, the starter blueprint deck, the ship aesthetic, and the console voice. The pipeline's "creative" step is replaced by *your prose*; the "constrained-choice" step still picks values from the existing enum option-space, now driven by what you wrote. See "Run Start — Authoring Your Civilization" above.
- **Encountered civs.** Generated by the existing pipeline (planet-driven, model-authored prose), pre-baked into a catalog so encounters don't cost an LLM call at runtime. Most civs you find are extinct — wrecks, ruins, dead beacons. Some are extant but distant, communicating only through signals. Real-time encounters are vanishingly rare and a major event when they happen.

Civs are gameplay-relevant via their tech tier: a tier-5 civ's drive in a tier-2 hull is the kind of breadcrumb that bends a run toward "get home." Your *own* civ's tier sets your starting distance from home — see "Distance is the Master Difficulty Scalar."

### Planets
Planets follow the existing minimal stub: type, gravity, atmosphere, temp, water, magnetosphere, star context. Planets are interesting because of what's *on* them (resources, ruins, biocompatibility) and what civ they birthed.

### Signals, Beacons, Wrecks
Three types of point-of-interest at the node level:
- **Signals** — radio sources. May be a civ broadcasting, a navigation beacon, a distress call, or noise. Interpreting them is a puzzle.
- **Beacons** — anchored point-of-interest markers, often left by long-dead civs as nav aids. Reading them grants partial sector maps.
- **Wrecks** — derelict ships. Salvage them for resources, blueprints, and sometimes a partial log of how they died. Very rarely, a wreck is *yours*.

## The Quest

### Getting Home
"Home" is a real coordinate in a real sector. At run start, your nav cannot point to it; you remember the *name* of home but not the chart. Reaching it requires:
- **Knowing where it is.** Breadcrumbs — beacons, recovered logs, signal triangulation — gradually populate the route.
- **Being able to get there.** Fuel range, drive class, hull integrity. Late-game blueprints may be required to make the final leg.
- **Choosing to.** Other endings remain available the whole time.

### Endings
The "Get Home" goal is **explicit**: a named objective on the captain's log from session one. The others are **emergent**, surfaced when their conditions are met:
- **Home.** You reached the named coordinate. The credits roll over your captain's log of the journey.
- **A new home.** You settled on a biocompatible planet with sufficient supplies and chose to stop. The captain's log records the colony.
- **Drift.** Out of fuel and out of options. The ship floats, life support runs out. Quiet ending.
- **Death.** Hull breach in deep space, catastrophic system failure, an unforgiving encounter. Sudden ending.

Endings are not failure states — except death and drift, which are. Settling is a chosen ending; reaching home is the named one. The captain's log treats all four with equal weight.

## Meta-Progression

### The Captain's Log
The captain's log is a persistent, cross-run record kept locally (and optionally synced). It contains:
- Every named place from every run (and who named it).
- Every blueprint ever collected.
- Every captain (you) — their civ, their ship's silhouette, their cause of ending.
- Statistics: total light-years traveled, civs encountered, probes lost, etc.

### Past-Self Encounters
Each new run spawns far from any prior run's path, in a different sector if possible. Encountering a past run's wreck is a rare event with a low base probability that scales with the number of past runs and the sectors they touched. When it happens, the encounter is treated as a major narrative beat: you find your old ship, you can recover its blueprints, you can read its final log.

### Blueprint Collection
Blueprints accumulate across runs as a *known set*. A new run does not start with every blueprint you've ever owned (that would trivialize starts); instead, your starting deck is drawn from your civ's tier-appropriate subset of the known set, plus a small chance for "remembered" blueprints from prior runs of similar civ.

## Systems Architecture

### What survives from the current build
- **`server/internal/llm/`** — kept as-is. Single-provider OpenAI client, structured-output mode.
- **`server/internal/factory/civgen.go`** + `planet.go` + civ struct — kept. Used to pre-bake the curated player-civ set and to generate encountered civs.
- **`factory/mixtures.go`** + `resources.go` — kept, demoted to flavor + blueprint input ontology. Mixtures are no longer simulated as a chemistry system; they're tags on resources and inputs to blueprint recipes.
- **`factory/assembly.go`** — repurposed for run-start ship rolling. Not used for in-run construction.
- **Go + Postgres + Three.js stack** — unchanged.

### What gets repurposed
- **`factory/system.go`** — becomes the sector + node-graph generator.
- **`handlers/ship.go`** — generalized into run-lifecycle handlers (start run, get current state, advance, end run).

### What gets cut
- **Multiplayer / shared global ledger.** No more shared universe. Each player's save is their own, with optional cloud sync of the captain's log.
- **Real-time orbital mechanics.** No keplerian trajectories, no maneuver nodes, no thruster simulation. Travel is "set a course, wait."
- **Signal propagation as physics.** Signals are content, not a simulated wave.
- **`factory/flight/`** as a player-facing system. May survive as internal helpers for procedural ship rolls.
- **Hardcore power/thermal/spectroscopy systems.** Replaced by the three meters and the four sensor-read modalities.

### New backend pieces
- **Sector graph** — generation, persistence, partial visibility per player.
- **Blueprint catalog** — the hidden recipe table, plus discovered/collected state.
- **Daily tick** — the log-scale-capped offline simulation. Runs on a cadence or on next-login, advances ship state by the elapsed time clamped to a max.
- **Captain's log persistence** — local-first; cloud sync is optional.

## Interface & Visuals

### Views
Three primary views, switchable from any session:
1. **Cockpit View** — first-person from the ship's bridge. You see out the window (stars, current node's bodies). Controls and console are in your field of view but minimal.
2. **Orbit View** — third-person camera circling your ship. Used for situational awareness, salvage operations, and admiring the procgen ship silhouette.
3. **Console View** — the working surface. Sector map, captain's log feed, probe reports, blueprint deck, sensor pane. Most decisions are made here.

### Aesthetic
- **Used-future / vintage-NASA.** Monochrome HUDs with one or two accent colors. CRT-style scanlines optional. Lots of negative space.
- **Low-poly procgen geometry.** Ships, planets, debris all built from a small set of primitives.
- **Sensor reads as art.** Spectrograms, point clouds, schematic diagrams. The "math is the art."
- **Quiet sound design.** Hum, occasional alerts, nothing musical unless earned.

## Mission Statement
$\exists$ exists to be a chill, contemplative, mathematically textured game about being alone in a very large place with a very specific desire — to get home, or to find a place to stop, or to drift. It rewards patience, reading carefully, and small good decisions. It refuses to reward reflex, optimization grinds, or the illusion that the universe cares.

What sets $\exists$ apart from its neighbors in the genre (FTL, Elite, Out There) is that *you author your own civilization* before the run begins. The ship you fly came from a people you wrote. The console speaks in their voice. The blueprints in your hand are the ones they would have given you. When you die, what dies is something you brought into being. The LLM is not a chatbot in the corner — it is the translator between your imagination and the game's mechanics.

Its mechanical promise: every session is short, every decision matters a little, and over weeks the small decisions accumulate into a journey worth remembering — about a people you made up, told the universe about, and watched the universe answer.

## Explicit Cuts from v1
- No multiplayer. No shared universe. No global ledger.
- No real-time flight controls. No orbital mechanics.
- No simulated signal propagation.
- No AI co-pilot personality.
- No hardcore power/thermal sim. Three meters instead.
- No pre-baked or hand-picked player civs — the player *authors* their civ at run start via a guided LLM conversation.
- No in-run ship swapping. One ship per run.
- No daily-cooldown after death — death just resets.
- No "easy mode" via authored advantage. Prose is gameplay-agnostic; capability buys reach, not power.
