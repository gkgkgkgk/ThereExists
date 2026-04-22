# Phase 5 Plan - Civilization Generation
*2026-04-21*

## Goal
Generate civilizations procedurally via an LLM pipeline that runs **Planet → Description → TechProfile + TechTier → Civilization**. Land the LLM infrastructure, a minimal `Planet` type sufficient to seed the pipeline, the civ-generation flow itself, and an admin endpoint that returns a fresh civ per call (no persistence, no pregeneration). Ships are not generated as part of civ generation — that's a later phase.

Phase 5 does **not** land the full planet generation system (Phase 5 `Planet` is a minimal stub — enough fields to inform civ generation, no biome simulation, no orbital mechanics, no system-level layout), does not persist civs to the DB, does not pregenerate catalogs, does not generate ships from civs, and does not replace the hand-authored `GenericCivilization` as a fallback. It is strictly: stand up the LLM client, define the minimal planet shape, wire the four-step generation pipeline, and expose it through an admin endpoint.

---

## 1. Why an LLM, and Where Creativity Lives

Civilizations are *almost* deterministic given their inputs. Planet conditions imply atmosphere and surface chemistry; atmosphere plus age implies technological trajectory; trajectory plus available chemistry implies preferred cooling methods, ignition types, and mixture families. The only genuinely creative leap is the **description** — the cultural / narrative layer that gives the civ texture.

So the LLM is used in two different modes across the pipeline:

- **Creative mode** — step 2 (description). Given a planet, write a short description. Open-ended prose, temperature > 0.
- **Constrained-choice mode** — step 3 (tech profile + tier). Given the planet and description and age, pick values from enumerated option sets. The LLM sees the entire option space (every mixture ID, every cooling method, every ignition method, tier 1-5) and returns selections. No free-form strings in this step except where the struct field is explicitly a string (e.g. `FarDriveFamily`).

This split matters because it determines prompt shape and validation. Creative steps get a soft prompt and we accept whatever comes back. Constrained steps get the full option menu inline and we validate the response against those options — any hallucinated mixture ID is a hard error, not a warning.

---

## 2. The Planet Stub

Phase 5 needs *just enough* planet to seed civ generation. Full planet generation is a later phase with its own pipeline. What lands now:

New file `server/internal/factory/planet.go`. Hand-authored struct, no registry (planets are generated, not registered):

- `ID string` — UUID-ish
- `Name string`
- `Type PlanetType` — new enum: `Terrestrial`, `OceanWorld`, `IceWorld`, `DesertWorld`, `GasGiant`, `LavaWorld` (small closed set; expand in the planet-generation phase)
- `SurfaceGravityG float64` — in g, 0.1 – 5.0 typical
- `AtmospherePressureAtm float64` — 0 = vacuum, 1 = Earth-like, 100+ = Venusian
- `AtmosphereComposition []string` — e.g. `["N2", "O2", "trace Ar"]`. Free-form strings; the LLM reads them, no enum validation in Phase 5.
- `SurfaceTempKRange [2]float64` — min/max in Kelvin (tidal / rotational variation)
- `HasLiquidWater bool`
- `HasMagnetosphere bool`
- `StarType string` — "G-type", "M-dwarf", "binary K+M", etc. Free-form.
- `OrbitalPeriodDays float64`

No moons, no resource deposits, no orbital mechanics, no biome map. Those live in the planet-generation phase.

### Generation in Phase 5
`planet.go` also ships a `GeneratePlanet(seed int64) (*Planet, error)` that produces a plausible planet **procedurally** — no LLM. This is the simplest path to unblock civ generation: a deterministic factory roll over reasonable ranges per `PlanetType`. When the full planet-generation phase lands, this function is replaced; the civ pipeline is insulated because it only depends on the struct shape.

### Scope guardrails
- No planet registry, no persistence.
- No LLM involvement in planet generation this phase (deferred).
- Atmosphere composition is free-form strings — no periodic-table enum yet.
- No validation beyond "field ranges are sane" (pressure ≥ 0, gravity > 0).

---

## 3. The LLM Package

New subpackage `server/internal/llm/`. Thin wrapper around the OpenAI Chat Completions / Responses API. Keeps the client concern out of `factory/`.

### Shape
- `llm.Client` — constructed from `OPENAI_API_KEY` env var. Reads key at construction, fails fast if missing.
- `llm.Complete(ctx, prompt, opts) (string, error)` — single-shot text response. Used for the creative description step.
- `llm.CompleteJSON(ctx, prompt, schema, out any) error` — structured-output variant. Prompt includes the JSON schema inline; response is parsed into `out`. Used for constrained-choice steps.
- Retries: one automatic retry on transient errors (5xx, timeout). Bail on 4xx.
- Timeout: 30s per call, configurable via option.
- Model: default `gpt-4o-mini` (cost-conscious for iteration); overridable per call.

### No caching, no batching, no streaming
Phase 5 generates one civ per request. Streaming, response caching, and batch pregeneration all defer to the pregeneration phase.

### Testability
The `Client` is an interface so the civ pipeline can accept a fake in tests. Tests never hit the real API — they inject canned responses and assert the pipeline handles parse errors, schema violations, and hallucinated IDs correctly.

### Scope guardrails
- One provider (OpenAI) — no abstraction over Claude / Gemini / local models.
- No token accounting, no cost tracking, no rate-limit backoff beyond one retry.
- No prompt templating engine — prompts are Go string formatters, inline with the step that uses them.

---

## 4. The Civilization Struct — Expansion

Existing `factory/civilizations.go` defines a thin `Civilization`. Phase 5 expands it and retains `GenericCivilization` as a fallback (not deleted — still useful for tests and deterministic ship rolls when the LLM is unavailable).

New fields:

- `Name string` — replaces the role currently played by `DisplayName`. Keep `DisplayName` as an alias field or rename outright; decision below in Open Questions.
- `Description string` — the creative-mode output. Multi-paragraph ok.
- `HomeworldDescription string` — short prose describing the homeworld conditions. Phase 5 derives this from the `Planet` stub; later phases replace with a `HomeworldPlanetID` reference.
- `HomeworldPlanetID *string` — nullable pointer, not populated in Phase 5 (no planet registry exists). Reserved so the struct shape survives the planet-generation phase without another migration.
- `AgeYears int64` — civilization age in years. Range: 100 – 10,000,000. Informs tech tier (older civs skew higher but not monotonically).
- `Flavor string` — keep, demoted. Short one-liner used in UI chips / summaries. Distinct from `Description`.

`TechTier` and `TechProfile` stay as they are. `TechProfile.FarDriveFamily` gets populated for the first time (Phase 4 left it empty on the generic civ).

### Backward compatibility
Old civ JSON lacking the new fields deserializes with zero-value defaults — JSONB is permissive. `FactoryVersion` bumps to `phase5-v1` regardless because the civ shape is a breaking content change.

---

## 5. The Generation Pipeline

New file `server/internal/factory/civgen.go`. Public entry point:

```go
func GenerateCivilization(ctx context.Context, client llm.Client, seed int64) (*Civilization, error)
```

Steps, in order. Each step's output feeds the next — sequential, not parallel.

### Step 1 — Planet (procedural)
`planet, err := GeneratePlanet(seed)`. Deterministic; no LLM.

### Step 2 — Description (LLM, creative)
Prompt the LLM with:
- The planet struct (marshaled as readable text)
- A brief instruction: "Write a 2–4 paragraph description of the civilization that evolved on this planet. Focus on culture, values, physical adaptations to the planet's conditions, and what makes them distinct. Do not discuss technology yet."
- Temperature ~0.9.

Output: raw text. Stored directly in `Civilization.Description`.

### Step 3 — Age (procedural, seeded)
`age := SampleAge(seed)`. Log-uniform over [100, 10_000_000] — most civs young, long tail of ancient ones. No LLM — this is a roll, not a judgment call.

### Step 4 — Tech profile and tier (LLM, constrained-choice)
Prompt the LLM with:
- The planet
- The description from step 2
- The age from step 3
- **The full option space**, inline:
  - Every mixture ID in `factory.Mixtures` (ID + one-line description)
  - Every `CoolingMethod` enum value
  - Every `IgnitionMethod` enum value
  - Tier 1-5 with short definitions
  - A short list of plausible `FarDriveFamily` strings ("RBCA", "fusion-torch", "solar-sail-relativistic", "none")
- An instruction: "Select values consistent with the civilization's conditions, culture, and age. Return JSON matching this schema: …"

The JSON schema names every field explicitly, including `PreferredMixtureIDs []string`, `PreferredCoolingMethods []string`, `PreferredIgnitionTypes []string`, `AversionToCryogenics float64 (0..1)`, `FarDriveFamily string`, `TechTier int (1..5)`.

Validation (hard errors, not warnings):
- Every `PreferredMixtureID` resolves in `factory.Mixtures`.
- Every `PreferredCoolingMethod` parses as a valid `CoolingMethod` enum.
- Every `PreferredIgnitionType` parses as a valid `IgnitionMethod` enum.
- `TechTier ∈ [1, 5]`.
- `AversionToCryogenics ∈ [0, 1]`.

On validation failure: one retry with the error fed back into the prompt. Second failure returns an error — the caller decides whether to bail or fall back to `GenericCivilization`.

### Step 5 — Name and Flavor (LLM, creative, cheap)
One final LLM call with the assembled civ-in-progress: produce a short `Name` (2-4 words, evocative) and a one-line `Flavor`. Separate from step 2 because step 2's prompt deliberately avoids naming to keep the description unanchored.

### Assemble
Populate `Civilization` struct from the five steps. Return.

---

## 6. The Endpoint

New handler `server/internal/handlers/civilization.go`. Mirrors `ship.go`'s shape but **does not require a player_id and does not persist**. Admin-only, same rationale as `/api/ships/generate` — internal testing, not a user-facing endpoint yet.

- `POST /api/civilizations/generate`
- Query params: `seed` (optional; defaults to time-based random), `model` (optional; OpenAI model override)
- Response: full `Civilization` JSON plus the `Planet` that seeded it (nested under a top-level `{ "civilization": ..., "planet": ... }` wrapper so the caller can see the inputs)
- No DB writes. Every call is fresh.

### On the player_id question
The current `/api/ships/generate` endpoint requires `player_id` for two reasons documented in `ship.go:39-88`: (a) to locate the player's active ship row for persistence, and (b) to default the seed from the player's row. Phase 5 does *not* refactor that endpoint — it's out of scope — but it does document the pattern for the civ endpoint: **factory generators are player-agnostic; persistence is the endpoint's concern**. When civs gain persistence in the pregeneration phase, the endpoint gets a persist variant; the generator stays pure.

---

## 7. FactoryVersion

Civilization shape change is breaking. Bump `FactoryVersion` to `phase5-v1`. No migration of existing rows.

---

## 8. Out of Scope (Explicit)

- **Ship generation from a civ.** The civ endpoint does not call the ship factory. Wiring civ preferences into ship rolls is a later phase.
- **Planet generation** beyond the stub. No biomes, no resource deposits, no moons, no system layout, no orbital mechanics.
- **Persistence of generated civs.** No DB schema, no `civilizations` table, no `civilization_id` column on `ships`.
- **Pregeneration / caching.** Every call hits the LLM. Batch pregen lands later.
- **Replacing `GenericCivilization`.** It stays as a fallback and test fixture.
- **Alternate LLM providers.** OpenAI only.
- **Prompt iteration tooling.** No eval harness, no A/B infrastructure, no prompt versioning. Inline prompts.
- **Client integration.** No UI to trigger civ generation, no display surface.
- **Authentication on the endpoint.** Mirrors ship endpoint — unauthenticated admin for now.
- **Cost tracking / budgets on LLM calls.**

---

## 9. Phase 5 Validation

End-of-phase acceptance:

- `POST /api/civilizations/generate` returns a valid `Civilization` + `Planet` JSON payload given a valid `OPENAI_API_KEY`.
- The pipeline runs Planet → Description → Age → TechProfile+Tier → Name+Flavor in order.
- Returned civ passes validation: all mixture IDs resolve, all cooling / ignition enums parse, tier ∈ [1,5], aversion ∈ [0,1].
- `server/internal/llm/` compiles and exposes a `Client` interface with `Complete` and `CompleteJSON`.
- `server/internal/factory/planet.go` defines `Planet`, `PlanetType`, and `GeneratePlanet(seed)` — the latter deterministic.
- `Civilization` struct gains `Name`, `Description`, `HomeworldDescription`, `HomeworldPlanetID`, `AgeYears`; existing civ data deserializes cleanly with zero-value new fields.
- One retry on validation failure in step 4; second failure returns an error.
- `GenericCivilization` still loads and is still selectable as a fallback.
- `FactoryVersion == "phase5-v1"`.

---

## 10. Open Questions

- **`DisplayName` vs `Name`.** Current struct has `DisplayName`; the natural field for LLM output is `Name`. Options: (a) rename `DisplayName` → `Name`, (b) keep both with `DisplayName` as the canonical and `Name` as an alias populated from it. Recommendation: rename. Phase 5 is a breaking factory bump anyway.
- **Age influencing tier.** Do we pass age as a nudge in the step-4 prompt ("older civs tend toward higher tier, but not always") or as a hard constraint? Recommendation: nudge. The LLM can override when the description implies stagnation or regression.
- **Starting `PlanetType` distribution.** Uniform across the six enum values, or weighted toward Terrestrial for variety's sake? Recommendation: uniform — we want the pipeline stressed on weird planets during iteration.
- **`FarDriveFamily` option list.** Hand-authored small set, or let the LLM freestyle? Recommendation: hand-authored — it's the hook for future Far-archetype gating, and a hallucinated family string breaks that gating.
- **Fallback to `GenericCivilization` on LLM failure.** Pipeline returns an error, caller decides. Should the *endpoint* fall back automatically, or surface the error? Recommendation: surface the error — admin endpoint, we want to see failures loudly.
- **Model default.** `gpt-4o-mini` is cheap but may fluff the description. `gpt-4o` is sharper. Start with mini, upgrade per-call via `?model=` if descriptions read thin.
