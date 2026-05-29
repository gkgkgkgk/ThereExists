# Phase 5 Implementation — Civilization Generation
*2026-04-21*

## Purpose
Translate `Phase5_Plan.md` into a concrete, commit-by-commit implementation plan detailed enough that a developer (or AI agent) with no prior context can execute Phase 5 end-to-end. Do not duplicate design rationale that already lives in the Plan doc — reference it. This doc answers **what code lands in what order and why**.

**Phase 5 is an LLM pipeline + a planet stub + an admin endpoint.** No persistence, no pregeneration, no ship-from-civ wiring, no UI.

---

## Prerequisites & starting state

**What exists today (just after merging `phase-4` into `main`, tip of `main`):**
- `server/internal/factory/civilizations.go` — thin `Civilization { ID, DisplayName, TechTier, TechProfile, Flavor }` with one registered `GenericCivilization` (tier 3). No `Name`, no `Description`, no age, no planet reference.
- `server/internal/factory/mixtures.go` — `Mixture` registry populated through Phase 4 / 4.1 with `IgnitionNeed`, `Synthetic`, `Precursors`, etc. This is the option space the step-4 LLM call will see.
- `server/internal/factory/enums.go` — `CoolingMethod`, `IgnitionMethod` enums with `String()` methods (used to build step-4 option menus).
- `server/internal/handlers/ship.go` — reference pattern for a factory-backed admin endpoint; Phase 5 mirrors its shape minus the `player_id` and minus persistence.
- `server/internal/factory/assembly/ship.go` — `FactoryVersion = "phase4_1-v1"` (or whatever the current tag is); bumps to `phase5-v1` at the end of Phase 5.
- No `server/internal/llm/` package.
- No `server/internal/factory/planet.go`.
- `OPENAI_API_KEY` in `.env` (user confirmed).

**Phase 5 lands:**
1. A new **`server/internal/llm/`** package with a `Client` interface, an OpenAI implementation, and a test fake. Two methods: `Complete` (creative) and `CompleteJSON` (structured, schema-validated).
2. A **`server/internal/factory/planet.go`** with a minimal `Planet` struct, `PlanetType` enum, and a deterministic `GeneratePlanet(seed int64)` procedural generator. No LLM.
3. **`Civilization` struct expansion** — `Name` (rename from `DisplayName`), `Description`, `HomeworldDescription`, `HomeworldPlanetID *string` (nil in Phase 5), `AgeYears int64`. `GenericCivilization` backfilled.
4. A **`server/internal/factory/civgen.go`** implementing the five-step pipeline (Planet → Description → Age → TechProfile+Tier → Name+Flavor). LLM client injected.
5. A **`POST /api/civilizations/generate`** admin endpoint returning `{ civilization, planet }`. No persistence. No `player_id`.
6. `FactoryVersion = "phase5-v1"`.
7. End-to-end test using the fake `llm.Client`.

**Out-of-scope** (Plan §8):
- Ship generation from a civ, planet generation beyond the stub, persistence, pregeneration / caching, replacing `GenericCivilization`, alternate LLM providers, prompt iteration tooling, client integration, endpoint auth, cost tracking.

---

## Branch strategy

Cut `phase-5` off `main` before commit 1. Each commit below is a real commit on `phase-5`. Keep them small and atomic. Merge to `main` only after the post-phase verification passes.

```
git checkout main
git pull
git checkout -b phase-5
```

---

## Commit-by-commit plan

### Commit 1 — `llm: Client interface + OpenAI impl + test fake`
**Intent.** Stand up the LLM package as a sibling to `handlers/` and `factory/`. Civ code doesn't exist yet; this commit is strictly infra. The package must be usable from tests without hitting the real API.

**Files created.**
- `server/internal/llm/client.go`:
  - `type Client interface`:
    ```
    Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
    CompleteJSON(ctx context.Context, prompt string, schema string, out any, opts ...Option) error
    ```
  - `type Option func(*callConfig)` — functional options for `Model`, `Temperature`, `MaxTokens`, `Timeout`. Defaults: model `gpt-4o-mini`, temperature `0.9` for `Complete` / `0.2` for `CompleteJSON`, timeout `30s`, no token cap.
  - `WithModel(string)`, `WithTemperature(float64)`, `WithTimeout(time.Duration)` constructors.
  - `ErrTransient` and `ErrValidation` sentinel errors for retry logic callers.
- `server/internal/llm/openai.go`:
  - `type OpenAIClient struct` — holds API key, HTTP client, default model.
  - `func NewOpenAIClient() (*OpenAIClient, error)` — reads `OPENAI_API_KEY` from env; returns error if missing.
  - `Complete` — single Chat Completions call. Returns the first message's content.
  - `CompleteJSON` — same but with `response_format: { type: "json_schema", json_schema: <inlined schema> }`. Parses result into `out` via `json.Unmarshal`. Parse failures → `ErrValidation`.
  - Retry policy inside the client: **one** automatic retry on 5xx or `context.DeadlineExceeded`. 4xx bail immediately.
- `server/internal/llm/fake.go`:
  - `type FakeClient struct` — programmable. Fields: `CompleteResponses []string`, `CompleteJSONResponses []string` (raw JSON strings), `CompleteErr error`, `CompleteJSONErr error`. Responses pop FIFO.
  - Implements `Client`.
  - Used in tests only. Build-tagged? **No** — no build tag. Fakes live alongside the real client so consumers can construct them freely in tests without import gymnastics.
- `server/internal/llm/client_test.go`:
  - Sanity tests for the `FakeClient` — pop order, error pass-through, JSON unmarshal into a struct.
  - **No tests that hit the real OpenAI API.** Live-call tests are a future concern.

**Gotchas.**
- Do not import `factory` from `llm` — `llm` is a leaf package. Civ pipeline imports both, not the other way around.
- The `schema` parameter on `CompleteJSON` is a raw JSON schema string, inlined into the API call. Keep it a string (not a `json.RawMessage` or a typed struct) so callers can `fmt.Sprintf` option menus into it without ceremony.
- Do not add streaming, batching, or caching. One request, one response, one optional retry. Anything more defers.
- `OPENAI_API_KEY` missing → `NewOpenAIClient` returns an error, not a panic. The server's startup path decides whether to fatal or fall back to `GenericCivilization`-only mode.

---

### Commit 2 — `factory: Planet stub + GeneratePlanet(seed)`
**Intent.** Land the minimal `Planet` type and a deterministic procedural generator. No LLM. No registry (planets are generated, not registered). Decoupled from civ generation so Phase 5's planet-generation-phase successor can swap the generator without touching civgen.

**Files created.**
- `server/internal/factory/planet.go`:
  - `type PlanetType int` with constants `Terrestrial`, `OceanWorld`, `IceWorld`, `DesertWorld`, `GasGiant`, `LavaWorld` + `String()`.
  - `type Planet struct` per Plan §2:
    ```
    ID                    string
    Name                  string
    Type                  PlanetType
    SurfaceGravityG       float64
    AtmospherePressureAtm float64
    AtmosphereComposition []string
    SurfaceTempKRange     [2]float64
    HasLiquidWater        bool
    HasMagnetosphere      bool
    StarType              string
    OrbitalPeriodDays     float64
    ```
  - `func GeneratePlanet(seed int64) (*Planet, error)`:
    - Seeds a `*rand.Rand`.
    - Uniform pick over the 6 `PlanetType` values.
    - Per-type parameter ranges hard-coded in a small table at the top of the file. E.g. `Terrestrial: gravity [0.3, 2.0], pressure [0.2, 3.0], tempK [200, 350], waterProb 0.8, magnetoProb 0.7`. `GasGiant: gravity [1.5, 5.0], pressure [50, 200], tempK [50, 200], waterProb 0.0, magnetoProb 0.95`. Etc.
    - `AtmosphereComposition` sampled from small per-type lists of plausible strings (`["N2", "O2"]`, `["CO2", "SO2"]`, `["H2", "He"]`, etc.).
    - `StarType` sampled from `["G-type", "K-type", "M-dwarf", "F-type", "binary K+M"]` uniformly.
    - `Name` generated from a small syllable table (no LLM). Deterministic per seed.
    - `ID` is `uuid.NewString()` — non-deterministic, but the rest of the struct is. (Call out: the ID is the only non-reproducible field. That's fine for Phase 5 since planets don't persist.)
  - `func (p *Planet) Describe() string` — renders a compact multi-line string representation suitable for inlining into an LLM prompt. Used by civgen step 2 and step 4.

**Files created (tests).**
- `server/internal/factory/planet_test.go`:
  - `TestGeneratePlanet_Deterministic` — same seed twice, every field except `ID` matches bit-for-bit.
  - `TestGeneratePlanet_FieldRanges` — 1000-seed sweep, every planet has `SurfaceGravityG > 0`, `AtmospherePressureAtm >= 0`, `SurfaceTempKRange[0] <= SurfaceTempKRange[1]`, `Type` in range, `Name` non-empty.

**Gotchas.**
- The per-type parameter table is load-bearing — sloppy ranges produce unphysical planets and the LLM will pattern-match on them. Keep ranges conservative and internally consistent (e.g. gas giants can't have liquid water, lava worlds can't have magnetospheres that survive the heat, etc. — encode in the per-type probabilities, don't try to enforce post-hoc).
- No `planets` registry, no `Planets` map, no `LookupPlanet`. `Planet` is a value type, not a registered singleton.
- `OrbitalPeriodDays` — keep ranges wide (`[30, 100000]`) but don't couple to `StarType` yet. That's planet-gen-phase work.
- Don't add biomes, moons, resource deposits, or orbital-mechanics fields. The struct in Plan §2 is the complete set.

---

### Commit 3 — `factory: Civilization struct expansion + GenericCivilization backfill`
**Intent.** Extend `Civilization` with the Phase 5 fields. Rename `DisplayName` → `Name` (Plan §10 open question, resolved: rename). Update `GenericCivilization` with the new required fields. Old JSON blobs deserialize cleanly into the new struct with zero-value new fields.

**Files modified.**
- `server/internal/factory/civilizations.go`:
  - Remove `DisplayName`. Add:
    ```
    Name                 string
    Description          string
    HomeworldDescription string
    HomeworldPlanetID    *string   // nil in Phase 5; reserved for planet-gen phase
    AgeYears             int64
    ```
  - Keep `Flavor` (demoted to short one-liner for UI chips per Plan §4).
  - Update `GenericCivilization` init:
    - `Name: "Generic Civilization"`.
    - `Description`: 2 short paragraphs describing a mid-tech spacefaring civilization with no strong biases. Hand-authored; not generated.
    - `HomeworldDescription`: one-line "temperate terrestrial homeworld, Earth-analogue."
    - `HomeworldPlanetID: nil`.
    - `AgeYears: 5000`.
    - `Flavor`: "Default mid-tier civ for development and fallback." (short)

**Files modified (if any reference `DisplayName`).**
- Grep the repo for `DisplayName` before this commit:
  - `Grep pattern:DisplayName`
  - Any UI/handler references switch to `Name`.
  - Any test fixtures update accordingly.

**Gotchas.**
- `HomeworldPlanetID *string` not `string` — nil is semantic. Non-nil means "this civ has a registered homeworld"; Phase 5 always leaves it nil because there's no planet registry.
- Do NOT populate `HomeworldDescription` from the `Planet` struct at struct-construction time. That's civgen's job (step 2 prompt produces it, or step 5 renders a short paraphrase). `GenericCivilization`'s value is hand-authored because it predates the pipeline.
- Old civ JSON in the DB (if any) may have `display_name` / `DisplayName` in it. JSONB is permissive — missing `name` field deserializes to empty string, which is fine for Phase 5 since we're bumping `FactoryVersion` anyway.
- `FactoryVersion` bump does NOT land in this commit. Holds until commit 6 so intermediate commits don't falsely advertise Phase 5 completeness.

---

### Commit 4 — `factory: civgen five-step pipeline + GenerateCivilization entry point`
**Intent.** The meat of Phase 5. Assemble the pipeline in `civgen.go` using the `llm.Client` from commit 1, the `Planet` from commit 2, and the expanded `Civilization` from commit 3. Prompts live inline (Plan §3 — no templating engine).

**Files created.**
- `server/internal/factory/civgen.go`:
  - Imports `llm`, `math/rand`, `context`, `fmt`, `strings`.
  - `func GenerateCivilization(ctx context.Context, client llm.Client, seed int64) (*Civilization, error)`:
    - **Step 1 — Planet.** `planet, err := GeneratePlanet(seed)`. Return on error.
    - **Step 2 — Description (creative LLM).**
      - Prompt built via `buildDescriptionPrompt(planet)`.
      - `description, err := client.Complete(ctx, prompt, llm.WithTemperature(0.9))`.
      - Return on error.
    - **Step 3 — Age (procedural).**
      - `age := sampleAgeYears(seed)` — log-uniform over `[100, 10_000_000]`. Implementation: sample `u` uniform on `[log(100), log(10_000_000)]`, return `int64(math.Exp(u))`.
    - **Step 4 — Tech profile + tier (constrained-choice LLM).**
      - Prompt built via `buildTechProfilePrompt(planet, description, age)`. Inlines:
        - Full mixture option list (ID + `Description` + `Synthetic` flag, from `factory.Mixtures`).
        - All `CoolingMethod` enum values + `String()`.
        - All `IgnitionMethod` enum values + `String()`.
        - Tier 1-5 one-line definitions (hand-authored constant block at top of `civgen.go`).
        - `FarDriveFamily` option set: `["RBCA", "fusion-torch", "solar-sail-relativistic", "none"]` (small hand-authored set per Plan §10).
      - Schema passed to `CompleteJSON`:
        ```
        {
          "type": "object",
          "required": ["preferred_mixture_ids", "preferred_cooling_methods",
                       "preferred_ignition_types", "aversion_to_cryogenics",
                       "far_drive_family", "tech_tier"],
          "properties": {
            "preferred_mixture_ids":    { "type": "array", "items": { "type": "string" } },
            "preferred_cooling_methods":{ "type": "array", "items": { "type": "string" } },
            "preferred_ignition_types": { "type": "array", "items": { "type": "string" } },
            "aversion_to_cryogenics":   { "type": "number", "minimum": 0, "maximum": 1 },
            "far_drive_family":         { "type": "string" },
            "tech_tier":                { "type": "integer", "minimum": 1, "maximum": 5 }
          }
        }
        ```
      - Deserialize into a private `techProfileResponse` struct.
      - Call `validateTechProfile(resp)`:
        - Every `preferred_mixture_ids` entry resolves in `factory.Mixtures`.
        - Every `preferred_cooling_methods` entry parses via a new `ParseCoolingMethod(string) (CoolingMethod, bool)` helper.
        - Every `preferred_ignition_types` entry parses via `ParseIgnitionMethod`.
        - `far_drive_family` is in the hand-authored option set.
        - `tech_tier ∈ [1,5]`.
        - `aversion_to_cryogenics ∈ [0,1]`.
      - **On validation failure: one retry.** Rebuild the prompt with an appended "Your previous response failed validation: \<error list\>. Return a corrected response." message. Re-call `CompleteJSON`. If the retry also fails validation, return an error.
    - **Step 5 — Name + Flavor (creative LLM, structured).**
      - Prompt built via `buildNameFlavorPrompt(planet, description, age, techProfile, techTier)`.
      - `CompleteJSON` with schema requiring `name` (string, 1-60 chars) and `flavor` (string, 1-140 chars).
      - No retry — if it fails, the caller decides.
    - **Assemble.**
      - `civ := &Civilization{ ID: uuid.NewString(), Name: r5.Name, Description: description, HomeworldDescription: extractHomeworldLine(description), HomeworldPlanetID: nil, AgeYears: age, TechTier: r4.TechTier, TechProfile: TechProfile{...}, Flavor: r5.Flavor }`.
      - `extractHomeworldLine` is a stub helper: returns the first sentence of `description` that mentions the planet's `Type.String()` or one of its `AtmosphereComposition` strings, else returns `fmt.Sprintf("Homeworld: %s with %s atmosphere.", planet.Type, strings.Join(planet.AtmosphereComposition, ", "))`. Imperfect, intentionally — full planet-gen phase replaces this with a structured homeworld field on `Planet`.
    - Return `civ, nil`.

**Files created (enums helpers).**
- `server/internal/factory/enums.go` — add `ParseCoolingMethod(string) (CoolingMethod, bool)` and `ParseIgnitionMethod(string) (IgnitionMethod, bool)` helpers if not already present. Map the enum `String()` output back to the enum value. Case-insensitive match to tolerate LLM casing drift.

**Files created (tests).**
- `server/internal/factory/civgen_test.go`:
  - `TestGenerateCivilization_HappyPath`:
    - Fake client with queued responses: description (raw string), tech profile JSON (valid), name+flavor JSON (valid).
    - Assert returned `Civilization` has correct fields populated, `AgeYears` in range, `TechTier ∈ [1,5]`.
  - `TestGenerateCivilization_ValidationRetry`:
    - First tech-profile response references a bogus mixture ID.
    - Second response is valid.
    - Assert civ is returned and `FakeClient.CompleteJSON` was called exactly twice for step 4.
  - `TestGenerateCivilization_ValidationHardFail`:
    - Both tech-profile responses invalid.
    - Assert error returned, error mentions which field failed.
  - `TestGenerateCivilization_DescriptionFailure`:
    - `Complete` returns `ErrTransient`.
    - Assert error bubbles; no step 3-5 calls happen.

**Gotchas.**
- Steps 1 and 3 are deterministic per seed. Steps 2, 4, 5 are LLM-driven and non-deterministic. Don't claim the whole pipeline is seed-deterministic — the planet and age are, but the civ's voice isn't. Note this in the function-level doc comment.
- The step-4 validator must use `LookupMixture` / `ParseCoolingMethod` / `ParseIgnitionMethod` — not string-equality against a hand-maintained list. If someone adds a new mixture, the validator picks it up for free.
- `extractHomeworldLine` is a known kludge. Flag it with a TODO comment referencing the planet-gen phase.
- Prompts are verbose. Put them behind `buildXPrompt` functions instead of inlining in `GenerateCivilization`. Each function returns a `string`. Keeps the entry point readable.
- JSON schema for `CompleteJSON` is passed as a raw string. Build the option menus by `fmt.Sprintf`-ing them into a template literal. Don't try to generate the schema from Go types — overkill for Phase 5.
- Do NOT hit the real OpenAI API in any test in this commit. Use `FakeClient` exclusively.

---

### Commit 5 — `handlers: POST /api/civilizations/generate`
**Intent.** Expose the civgen pipeline through an admin endpoint. Mirrors `ship.go`'s handler shape but omits `player_id` and omits persistence — both out of scope per Plan §6.

**Files created.**
- `server/internal/handlers/civilization.go`:
  - `type CivilizationHandler struct { llm llm.Client }`.
  - `func NewCivilizationHandler(client llm.Client) *CivilizationHandler`.
  - Handler method `Generate`:
    - Parse optional `seed` query param; default to `time.Now().UnixNano()`.
    - Parse optional `model` query param; pass via `llm.WithModel` to the client's per-call options. (**Caveat:** commit 4 doesn't plumb per-call model overrides into `GenerateCivilization`. Either thread a `...Option` variadic through `GenerateCivilization` in this commit, or accept the limitation and drop `?model=`. Recommended: add the variadic in commit 4 so the handler can pass it through cleanly; cross-reference this commit's scope.)
    - Call `factory.GenerateCivilization(r.Context(), h.llm, seed)`.
    - On error, `500` with a logged stack. Do NOT fall back to `GenericCivilization` — per Plan §10, the admin endpoint surfaces failures loudly.
    - On success, marshal `{ "civilization": <civ>, "planet": <the planet that seeded it> }`. **Planet exposure:** the current `GenerateCivilization` signature returns only `*Civilization`. Either:
      - (a) return `(*Civilization, *Planet, error)` — recommended, minor change;
      - (b) regenerate the planet from the seed in the handler via `GeneratePlanet(seed)` (deterministic, so same planet).
      - Go with (a). Simpler and avoids a wasted procedural roll.
    - Swagger annotations mirror `ship.go:28-38` style.
  - Route: `POST /api/civilizations/generate` — wired in whatever file registers routes (likely `server/cmd/server/main.go` or equivalent — grep for `ship.*Generate`).

**Files modified.**
- Wherever routes are registered: add the civ handler alongside ship. Construct the `llm.Client` at startup: `client, err := llm.NewOpenAIClient(); if err != nil { log.Printf("OPENAI_API_KEY missing: civ endpoint will 500"); client = nil }`. Handler constructor accepts a nil client; `Generate` 503s early if nil rather than panicking. This keeps the server bootable without the key for non-civ work.

**Files created (tests).**
- `server/internal/handlers/civilization_test.go`:
  - `TestCivilizationHandler_Success` — inject fake client, hit endpoint, assert 200 and JSON shape `{ civilization, planet }`.
  - `TestCivilizationHandler_LLMError` — fake client returns error, assert 500.
  - `TestCivilizationHandler_MissingKey` — handler constructed with nil client, assert 503.

**Gotchas.**
- Do NOT accept `player_id` as a query param. The endpoint is player-agnostic.
- Do NOT persist. No DB writes. If future-me is tempted to "just stash it in a `civilizations` table real quick" — that's the pregen phase, not Phase 5.
- The 503 on missing key is intentional. Failing loudly > silently falling back to `GenericCivilization`.
- Swagger/docs regen: if the project has a `swag init` step (see `server/api/docs.go`, `swagger.json`, `swagger.yaml`), run it as part of this commit so the generated docs reflect the new endpoint.

---

### Commit 6 — `factory/llm: FactoryVersion bump + end-to-end smoke test`
**Intent.** Tag Phase 5 as shipped. Add a tripwire test that exercises the full pipeline with a fake client through the handler layer.

**Files modified.**
- `server/internal/factory/assembly/ship.go` — `const FactoryVersion = "phase5-v1"`.

**Files created.**
- `server/internal/handlers/civilization_e2e_test.go`:
  - `TestCivgenEndToEnd`:
    - Construct a `FakeClient` with three queued responses (description, tech profile, name+flavor).
    - Construct `CivilizationHandler` with the fake.
    - `httptest.NewServer` the route.
    - `POST /api/civilizations/generate?seed=42`.
    - Assert 200, response JSON has `civilization` and `planet` keys, `civilization.tech_tier ∈ [1,5]`, `civilization.age_years > 0`, `planet.type` non-empty.
    - Second request with the same seed: `planet` fields (except `id`) match bit-for-bit; `civilization` fields may differ (LLM responses are queued, different order of consumption would differ — but here the fake is reset per-test).
  - `TestCivgenFactoryVersion` — trivial assertion `FactoryVersion == "phase5-v1"`.

**Gotchas.**
- `FactoryVersion` bump is the final commit because intermediate commits 1-5 don't yet constitute Phase 5 completeness; advertising `phase5-v1` mid-phase poisons any ships generated during development.
- Do not regenerate ships in this test. Phase 5 doesn't change ship generation; the sweep test from Phase 4 (`ship_sweep_test.go`) still passes unchanged — verify that locally before the commit.

---

## Post-phase verification (manual)

1. Fresh DB. Server up. `OPENAI_API_KEY` set in env.
2. `go test ./server/...` — all green. Zero real OpenAI calls in the test suite (grep `openai.com` in test logs to confirm).
3. `curl -X POST "http://localhost:8080/api/civilizations/generate?seed=1"`:
   - Response shape: `{ "civilization": {...}, "planet": {...} }`.
   - `civilization.name` non-empty.
   - `civilization.description` is multi-paragraph prose (>= 200 chars).
   - `civilization.tech_tier ∈ [1,5]`.
   - Every `civilization.tech_profile.preferred_mixture_ids` entry resolves in `factory.Mixtures` (spot-check 2-3).
   - `civilization.homeworld_planet_id` is `null`.
   - `planet.type` is one of the 6 enum values.
4. Repeat with `seed=1`: `planet` matches bit-for-bit (except `id`); civilization differs (LLM non-determinism — expected).
5. `curl -X POST "http://localhost:8080/api/civilizations/generate?seed=1&model=gpt-4o"` — response returns, uses the overridden model (spot-check via OpenAI dashboard if available; otherwise trust the plumbing and add a log line at DEBUG level).
6. Unset `OPENAI_API_KEY`, restart server, hit the endpoint: 503 with a clear error message. Server still boots.
7. Hit `/api/ships/generate` with a valid `player_id`: still works, `factory_version == "phase5-v1"` on the returned payload.
8. Confirm `grep -r "DisplayName" server/` returns nothing (rename complete).
9. Confirm `server/internal/llm/` and `server/internal/factory/planet.go` exist; `server/internal/factory/civgen.go` exists.

---

## Deferred-but-spec'd items (extension points)

- **Persistence.** `civilizations` table, `civilization_id` FK on `ships`, `GET /api/civilizations/:id`, pregeneration batch job.
- **Ship-from-civ generation.** The `GenerateRandomShip` seed path grows a `civID` parameter; the civ's `TechProfile` biases archetype selection; the civ's `TechTier` gates Far-slot availability (Phase 4 already supports this, just needs a non-generic civ source).
- **Full planet generation.** Replaces `GeneratePlanet` with its own LLM pipeline; adds biomes, moons, resource deposits, system layout.
- **`HomeworldPlanetID`** becomes non-nil once planets persist; `HomeworldDescription` becomes a rendered view of the referenced planet rather than free-form text.
- **Multi-provider LLM support.** `llm.Client` interface already supports this — add `AnthropicClient`, `LocalClient`, etc.
- **Prompt versioning / eval harness.** Track which prompt version generated which civ; run A/B evals on description quality.
- **Rate limiting and cost tracking.** Token accounting, per-call cost logging, budget ceilings.
- **Replacing `GenericCivilization`** once pregen catalog is populated — or keeping it purely as a test fixture.
- **Player-agnostic ship endpoint variant.** Drop `player_id` requirement on `/api/ships/generate` once refactored into a pure "roll a loadout" + separate "persist to player" pair. Flagged for future cleanup.

---

## Summary of what lands

A new `llm` package with a `Client` interface, OpenAI implementation, and test fake; a minimal `Planet` struct + deterministic `GeneratePlanet(seed)` procedural generator; an expanded `Civilization` struct with `Name`, `Description`, `HomeworldDescription`, `HomeworldPlanetID` (nil), `AgeYears`; a five-step civgen pipeline (Planet → Description → Age → TechProfile+Tier → Name+Flavor) with one-retry validation on the constrained-choice step; a `POST /api/civilizations/generate` admin endpoint returning `{ civilization, planet }` with no persistence and no `player_id`; `FactoryVersion = "phase5-v1"`; end-to-end tests using the fake LLM client. No real OpenAI calls in the test suite. `GenericCivilization` retained as a fallback and test fixture.
