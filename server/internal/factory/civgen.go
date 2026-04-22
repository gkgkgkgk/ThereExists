package factory

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// Tier definitions inlined into the step-4 prompt so the LLM sees
// consistent semantics for the 1–5 scale.
const techTierMenu = `Tech tiers:
  1 — Pre-spacefaring. Chemical rockets only. No orbital infrastructure.
  2 — Early spacefaring. Reliable LEO and lunar operations. No interplanetary crewed missions.
  3 — Mature spacefaring. Routine interplanetary operations, mature propellant synthesis. The current median.
  4 — Advanced. Fusion-adjacent propulsion, long-duration crewed missions across a star system. Routine refinery automation.
  5 — Relativistic. Matter/antimatter or equivalent exotic drives. Capable of interstellar transit at a meaningful fraction of c.`

// FarDriveFamily option set. Hand-authored and small so step-4 can't
// hallucinate a family string that breaks future Far-archetype gating.
var farDriveFamilies = []string{"RBCA", "fusion-torch", "solar-sail-relativistic", "none"}

// GenerateCivilization runs the five-step Phase 5 pipeline. Steps 1 and
// 3 are deterministic per seed; steps 2, 4, and 5 are LLM-driven and
// non-deterministic even with the same seed. Returns the civ and the
// planet that seeded it so the caller can expose both (the planet would
// otherwise be unrecoverable — its UUID is fresh per GeneratePlanet
// call).
func GenerateCivilization(ctx context.Context, client llm.Client, seed int64, opts ...llm.Option) (*Civilization, *Planet, error) {
	// Step 1 — Planet (procedural).
	planet, err := GeneratePlanet(seed)
	if err != nil {
		return nil, nil, fmt.Errorf("civgen step 1 (planet): %w", err)
	}

	// Step 2 — Description (LLM, creative).
	descPrompt := buildDescriptionPrompt(planet)
	description, err := client.Complete(ctx, descPrompt, append(opts, llm.WithTemperature(0.9))...)
	if err != nil {
		return nil, nil, fmt.Errorf("civgen step 2 (description): %w", err)
	}
	description = strings.TrimSpace(description)

	// Step 3 — Age (procedural, seeded).
	age := sampleAgeYears(seed)

	// Step 4 — Tech profile + tier (LLM, constrained-choice, 1 retry).
	profile, err := generateTechProfile(ctx, client, planet, description, age, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("civgen step 4 (tech profile): %w", err)
	}

	// Step 5 — Name + Flavor (LLM, structured, no retry).
	nameFlavor, err := generateNameFlavor(ctx, client, planet, description, age, profile, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("civgen step 5 (name/flavor): %w", err)
	}

	civ := &Civilization{
		ID:                   uuid.NewString(),
		Name:                 strings.TrimSpace(nameFlavor.Name),
		Description:          description,
		HomeworldDescription: extractHomeworldLine(description, planet),
		HomeworldPlanetID:    nil, // reserved for planet-gen phase
		AgeYears:             age,
		TechTier:             profile.TechTier,
		TechProfile:          profile.ToTechProfile(),
		Flavor:               strings.TrimSpace(nameFlavor.Flavor),
	}
	return civ, planet, nil
}

// sampleAgeYears returns a log-uniform age in [100, 10_000_000]. Most
// civs are young; a long tail of ancient ones. Seeded so repeated calls
// at the same seed return the same age — the only deterministic piece
// of civ metadata besides the planet.
func sampleAgeYears(seed int64) int64 {
	// Separate stream from planet rolls so planet changes don't shift
	// age unpredictably when we tune the planet generator.
	r := rand.New(rand.NewSource(seed ^ 0x5A6EA5E))
	lo := math.Log(100)
	hi := math.Log(10_000_000)
	u := lo + r.Float64()*(hi-lo)
	return int64(math.Exp(u))
}

// ──────────────────────── Step 2: description ────────────────────────

func buildDescriptionPrompt(p *Planet) string {
	return fmt.Sprintf(`You are worldbuilding a civilization that evolved on the following planet.

%s

Write a 2–4 paragraph description of the civilization. Focus on culture, values, physical or social adaptations to the planet's conditions, and what makes them distinct from other spacefaring peoples. Do NOT discuss their technology level, their propulsion preferences, or their age yet — those are determined separately. Do NOT give them a name.

Prose only. No headings, no lists.`, p.Describe())
}

// ──────────────────────── Step 4: tech profile ────────────────────────

type techProfileResponse struct {
	PreferredMixtureIDs     []string `json:"preferred_mixture_ids"`
	PreferredCoolingMethods []string `json:"preferred_cooling_methods"`
	PreferredIgnitionTypes  []string `json:"preferred_ignition_types"`
	AversionToCryogenics    float64  `json:"aversion_to_cryogenics"`
	FarDriveFamily          string   `json:"far_drive_family"`
	TechTier                int      `json:"tech_tier"`
}

// ToTechProfile converts the LLM-returned shape into the domain struct.
// Safe to call only after validateTechProfile has passed.
func (r techProfileResponse) ToTechProfile() TechProfile {
	cooling := make([]CoolingMethod, 0, len(r.PreferredCoolingMethods))
	for _, s := range r.PreferredCoolingMethods {
		if c, ok := ParseCoolingMethod(s); ok {
			cooling = append(cooling, c)
		}
	}
	ignition := make([]IgnitionMethod, 0, len(r.PreferredIgnitionTypes))
	for _, s := range r.PreferredIgnitionTypes {
		if i, ok := ParseIgnitionMethod(s); ok {
			ignition = append(ignition, i)
		}
	}
	return TechProfile{
		PreferredCoolingMethods: cooling,
		PreferredIgnitionTypes:  ignition,
		PreferredMixtureIDs:     r.PreferredMixtureIDs,
		AversionToCryogenics:    r.AversionToCryogenics,
		FarDriveFamily:          r.FarDriveFamily,
	}
}

func generateTechProfile(ctx context.Context, client llm.Client, planet *Planet, description string, age int64, opts []llm.Option) (techProfileResponse, error) {
	prompt := buildTechProfilePrompt(planet, description, age, "")
	schema := techProfileSchema()

	var resp techProfileResponse
	err := client.CompleteJSON(ctx, prompt, schema, &resp, append(opts, llm.WithTemperature(0.2))...)
	if err == nil {
		if verr := validateTechProfile(resp); verr == nil {
			return resp, nil
		} else {
			// One retry with the validation error fed back.
			retryPrompt := buildTechProfilePrompt(planet, description, age, verr.Error())
			var retry techProfileResponse
			if rerr := client.CompleteJSON(ctx, retryPrompt, schema, &retry, append(opts, llm.WithTemperature(0.2))...); rerr != nil {
				return techProfileResponse{}, fmt.Errorf("retry after validation failure: %w (original validation error: %v)", rerr, verr)
			}
			if verr2 := validateTechProfile(retry); verr2 != nil {
				return techProfileResponse{}, fmt.Errorf("validation failed twice: first=%v, second=%v", verr, verr2)
			}
			return retry, nil
		}
	}
	return techProfileResponse{}, err
}

func buildTechProfilePrompt(p *Planet, description string, age int64, priorValidationError string) string {
	var b strings.Builder

	b.WriteString("You are assigning technical preferences to a civilization. Their planet and culture are given; you must select values consistent with both.\n\n")
	b.WriteString("PLANET:\n")
	b.WriteString(p.Describe())
	b.WriteString("\nCULTURAL DESCRIPTION:\n")
	b.WriteString(description)
	fmt.Fprintf(&b, "\n\nAGE: %d years. (Older civs tend toward higher tech tiers, but not monotonically — stagnation and regression happen.)\n\n", age)

	b.WriteString("MIXTURE OPTIONS (use IDs exactly as shown):\n")
	for _, id := range sortedMixtureIDs() {
		m := Mixtures[id]
		tags := []string{}
		if m.Hypergolic {
			tags = append(tags, "hypergolic")
		}
		if m.Cryogenic {
			tags = append(tags, "cryogenic")
		}
		if m.Synthetic {
			tags = append(tags, "synthetic")
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		fmt.Fprintf(&b, "  - %s%s: %s\n", m.ID, tagStr, m.Description)
	}

	b.WriteString("\nCOOLING METHOD OPTIONS: ")
	b.WriteString(joinEnum(AllCoolingMethods))
	b.WriteString("\nIGNITION METHOD OPTIONS: ")
	b.WriteString(joinEnum(AllIgnitionMethods))
	b.WriteString("\nFAR DRIVE FAMILY OPTIONS (pick exactly one): ")
	b.WriteString(strings.Join(farDriveFamilies, ", "))
	b.WriteString("\n\n")
	b.WriteString(techTierMenu)
	b.WriteString("\n\n")

	b.WriteString("Return a JSON object with the following fields:\n")
	b.WriteString("  preferred_mixture_ids:     array of mixture IDs the civ favors (1–4 entries)\n")
	b.WriteString("  preferred_cooling_methods: array of cooling method names (1–3 entries)\n")
	b.WriteString("  preferred_ignition_types:  array of ignition method names (1–3 entries)\n")
	b.WriteString("  aversion_to_cryogenics:    float in [0,1] — 0 means no aversion, 1 means they refuse cryogenic mixtures entirely\n")
	b.WriteString("  far_drive_family:          one string from the Far Drive Family options above\n")
	b.WriteString("  tech_tier:                 integer 1–5\n")
	b.WriteString("\nOnly use IDs and enum values exactly as shown. Do not invent new ones.\n")

	if priorValidationError != "" {
		b.WriteString("\nYour previous response failed validation: ")
		b.WriteString(priorValidationError)
		b.WriteString("\nReturn a corrected response using only the listed options.\n")
	}

	return b.String()
}

func techProfileSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "required": ["preferred_mixture_ids", "preferred_cooling_methods", "preferred_ignition_types", "aversion_to_cryogenics", "far_drive_family", "tech_tier"],
  "properties": {
    "preferred_mixture_ids":     { "type": "array", "items": { "type": "string" } },
    "preferred_cooling_methods": { "type": "array", "items": { "type": "string" } },
    "preferred_ignition_types":  { "type": "array", "items": { "type": "string" } },
    "aversion_to_cryogenics":    { "type": "number" },
    "far_drive_family":          { "type": "string" },
    "tech_tier":                 { "type": "integer" }
  }
}`
}

func validateTechProfile(r techProfileResponse) error {
	var errs []string

	if r.TechTier < 1 || r.TechTier > 5 {
		errs = append(errs, fmt.Sprintf("tech_tier %d is outside [1,5]", r.TechTier))
	}
	if r.AversionToCryogenics < 0 || r.AversionToCryogenics > 1 {
		errs = append(errs, fmt.Sprintf("aversion_to_cryogenics %.3f is outside [0,1]", r.AversionToCryogenics))
	}
	if len(r.PreferredMixtureIDs) == 0 {
		errs = append(errs, "preferred_mixture_ids is empty")
	}
	for _, id := range r.PreferredMixtureIDs {
		if _, ok := LookupMixture(id); !ok {
			errs = append(errs, fmt.Sprintf("mixture id %q does not resolve", id))
		}
	}
	for _, s := range r.PreferredCoolingMethods {
		if _, ok := ParseCoolingMethod(s); !ok {
			errs = append(errs, fmt.Sprintf("cooling method %q is not a valid enum value", s))
		}
	}
	for _, s := range r.PreferredIgnitionTypes {
		if _, ok := ParseIgnitionMethod(s); !ok {
			errs = append(errs, fmt.Sprintf("ignition method %q is not a valid enum value", s))
		}
	}
	if !slices.Contains(farDriveFamilies, r.FarDriveFamily) {
		errs = append(errs, fmt.Sprintf("far_drive_family %q is not one of: %s", r.FarDriveFamily, strings.Join(farDriveFamilies, ", ")))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// ──────────────────────── Step 5: name + flavor ────────────────────────

type nameFlavorResponse struct {
	Name   string `json:"name"`
	Flavor string `json:"flavor"`
}

func generateNameFlavor(ctx context.Context, client llm.Client, planet *Planet, description string, age int64, profile techProfileResponse, opts []llm.Option) (nameFlavorResponse, error) {
	prompt := fmt.Sprintf(`Given the following civilization profile, produce a short evocative name and a one-line flavor string.

PLANET:
%s
DESCRIPTION:
%s

AGE: %d years
TECH TIER: %d
FAR DRIVE FAMILY: %s

Name: 2–4 words, evocative, culture-appropriate. Not generic.
Flavor: a single sentence (max ~140 characters) suitable for a UI chip or summary line.`,
		planet.Describe(), description, age, profile.TechTier, profile.FarDriveFamily)

	schema := `{
  "type": "object",
  "additionalProperties": false,
  "required": ["name", "flavor"],
  "properties": {
    "name":   { "type": "string" },
    "flavor": { "type": "string" }
  }
}`

	var out nameFlavorResponse
	if err := client.CompleteJSON(ctx, prompt, schema, &out, append(opts, llm.WithTemperature(0.8))...); err != nil {
		return nameFlavorResponse{}, err
	}
	if out.Name == "" {
		return nameFlavorResponse{}, fmt.Errorf("empty name")
	}
	if out.Flavor == "" {
		return nameFlavorResponse{}, fmt.Errorf("empty flavor")
	}
	return out, nil
}

// ──────────────────────── helpers ────────────────────────

func sortedMixtureIDs() []string {
	ids := make([]string, 0, len(Mixtures))
	for id := range Mixtures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func joinEnum[T fmt.Stringer](vals []T) string {
	s := make([]string, len(vals))
	for i, v := range vals {
		s[i] = v.String()
	}
	return strings.Join(s, ", ")
}

// sentenceSplitter is intentionally dumb — splits on ". ", "! ", "? ".
// Good enough for extracting a "homeworld-sentence" from a short
// description; not a general-purpose segmenter.
var sentenceSplitter = regexp.MustCompile(`[.!?]\s+`)

// extractHomeworldLine returns the first sentence of the description
// that mentions the planet type or one of its atmospheric species, else
// a templated fallback. Known kludge — replaced once planet-gen lands a
// structured homeworld field on Planet.
// TODO(planet-gen): replace with a structured field on Planet that civgen
// renders directly.
func extractHomeworldLine(description string, p *Planet) string {
	needles := []string{p.Type.String()}
	// Match on word fragments inside the String() form (e.g. "ocean"
	// from "ocean_world") — the description won't contain the snake-
	// case enum literal.
	for _, tok := range strings.Split(p.Type.String(), "_") {
		if len(tok) >= 4 {
			needles = append(needles, tok)
		}
	}
	needles = append(needles, p.AtmosphereComposition...)
	if p.HasLiquidWater {
		needles = append(needles, "water", "ocean", "sea")
	}

	for _, sentence := range sentenceSplitter.Split(description, -1) {
		low := strings.ToLower(sentence)
		for _, n := range needles {
			if n == "" {
				continue
			}
			if strings.Contains(low, strings.ToLower(n)) {
				return strings.TrimSpace(sentence)
			}
		}
	}

	return fmt.Sprintf("Homeworld is a %s with %s atmosphere.",
		strings.ReplaceAll(p.Type.String(), "_", " "),
		strings.Join(p.AtmosphereComposition, ", "))
}
