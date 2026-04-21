# Time Dilation

This document describes how relativistic travel is surfaced to the player in There Exists. It is a design-layer document — no runtime behaviour is implemented in Phase 4. Phase 4 ships the factory infrastructure for Far-category drives (RBCA, Matter/Antimatter, TechTier 5 gate). A later phase adds the two-clock simulation described here.

## Why time dilation exists in the game

Far-category drives push ships to a meaningful fraction of c. At 0.85c (RBCA baseline), the Lorentz factor γ ≈ 1.90 — one hour of ship time costs ~1.9 hours of universe time. This is the defining gameplay texture of the Far slot: the ship moves between stars fast, but the universe keeps moving while it's gone. A crew that leaves for a week-long transit comes back to a station that aged a month. Contracts expire. Cargo prices move. People die.

The intent is not to simulate special relativity for its own sake — it's to make interstellar travel feel *costly in the dimension that matters to the player* (wall-clock time, social consequence) rather than just fuel-costly. A ship with infinite fuel but a 30% c top speed is still a ship that can only show up to half the parties it's invited to.

## Two-clock model

At any moment the sim tracks two clocks for every ship:

- **Proper time** (`τ`): time experienced on the ship. Advances by `dτ` each simulation tick. All per-ship processes — life support, crew aging, equipment wear, mission timers, the player's clock while flying manually — tick on `τ`.
- **Coordinate time** (`t`): time in the universe's shared frame. All inter-ship events, contract deadlines, market ticks, NPC schedules, station timers, and the player's clock *while not flying* tick on `t`.

The relationship is `dt = γ · dτ`, where `γ = 1 / sqrt(1 - (v/c)²)` and `v` is the ship's speed in the coordinate frame. At rest in the coordinate frame (`v = 0`, `γ = 1`) both clocks advance at the same rate — there's no dilation when the ship isn't moving. At 0.85c, 1 hour of `τ` is ~1.9 hours of `t`.

Only the Far slot produces velocities where `γ` departs noticeably from 1. Short and Medium flight runs at rounding error on a stellar scale; we clamp `γ = 1` for them so the two clocks stay in lockstep during everyday flight and the divergence is a feature of Far specifically.

## What the player sees

The player's HUD shows a single clock — `τ` while they are aboard a ship that's moving relativistically, `t` while they're on a station or anywhere else. A small indicator ("γ 1.9" or "1h → 1.9h universe time") makes the stretch legible during Far transits. When the player arrives at the destination, they see the universe-time delta that elapsed and any events that resolved in their absence (contracts that expired, prices that moved, messages that queued up).

The goal is legibility, not accuracy: a player should never need to know what a Lorentz factor is. They should know "I was gone longer than I thought."

## What changes downstream when this lands

- **Mission timers**: every authored deadline is `t`-denominated. A 72-hour delivery contract is 72 coordinate-hours. The player's choice of which drive to use is directly a choice of how much `τ` they'll burn getting there.
- **Life support / consumables**: `τ`-denominated. A week of rations covers a week of ship time regardless of how much coordinate time passes outside.
- **Crew / wear**: `τ`-denominated. Crews don't age faster because the ship is moving; they age *slower than the universe*.
- **Market / NPC simulation**: `t`-denominated. Everything off-ship keeps running at coordinate time.

The split is "on-ship processes use τ, off-ship processes use t." That rule is small enough to audit and hard enough to break in interesting ways that we want to codify it early.

## Not in Phase 4

- No γ calculation at runtime.
- No separate `τ` and `t` fields on any entity.
- No HUD indicator.
- No consumable / contract clocks split by frame.

Phase 4 only ships the drives *capable* of producing relativistic velocities. The two-clock simulation, the clock-split audit across contracts/consumables/markets, and the HUD are future work.

## Open questions

- **Frame of reference for Short/Medium in-system flight.** Everyday in-system travel stays non-relativistic by construction (Short/Medium drives top out far below c). We clamp γ = 1 for those slots, but "how does the switch happen when a ship transitions between slot regimes" is a detail the runtime phase will have to answer.
- **Communications during Far transit.** If the ship is at 0.85c between stars, light-speed messages still exist, but Doppler and time-of-flight matter. Probably out of scope for the first pass — messages queue at the destination — but worth flagging.
- **Player expectation management.** The one-clock → two-clock handoff is a UI problem disguised as a physics problem. Getting the first Far transit to feel "oh, interesting" rather than "wait, what?" is a tutorial / onboarding decision we haven't made yet.
