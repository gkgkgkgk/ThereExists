package factory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// validTechProfileJSON returns a JSON blob that passes validation
// against the current registries. Picks the first mixture ID in the
// sorted list to avoid coupling to registry ordering.
func validTechProfileJSON(t *testing.T) string {
	t.Helper()
	ids := sortedMixtureIDs()
	if len(ids) == 0 {
		t.Fatal("no mixtures registered — test precondition violated")
	}
	return `{
  "preferred_mixture_ids": ["` + ids[0] + `"],
  "preferred_cooling_methods": ["regenerative"],
  "preferred_ignition_types": ["spark"],
  "aversion_to_cryogenics": 0.2,
  "far_drive_family": "none",
  "tech_tier": 3
}`
}

const validNameFlavorJSON = `{"name":"Thelassar Drift","flavor":"Quiet oceanic engineers who treat ships as living things."}`

const validDescriptionJSON = `{"description":"They are a thoughtful people who build ships slowly and trust old catalysts. Their cities are terraced along thermal ridges.","design_philosophy":"ritualised conservatism; nothing unproven flies"}`

func TestGenerateCivilization_HappyPath(t *testing.T) {
	fake := &llm.FakeClient{
		CompleteJSONResponses: []string{validDescriptionJSON, validTechProfileJSON(t), validNameFlavorJSON},
	}

	civ, planet, err := GenerateCivilization(context.Background(), fake, 42)
	if err != nil {
		t.Fatalf("GenerateCivilization: %v", err)
	}
	if civ == nil || planet == nil {
		t.Fatalf("nil civ or planet: civ=%v planet=%v", civ, planet)
	}
	if civ.Name == "" {
		t.Error("empty Name")
	}
	if civ.Description == "" {
		t.Error("empty Description")
	}
	if civ.Flavor == "" {
		t.Error("empty Flavor")
	}
	if civ.TechTier < 1 || civ.TechTier > 5 {
		t.Errorf("TechTier %d out of range", civ.TechTier)
	}
	if civ.AgeYears < 100 || civ.AgeYears > 10_000_000 {
		t.Errorf("AgeYears %d out of range", civ.AgeYears)
	}
	if civ.HomeworldPlanetID != nil {
		t.Error("HomeworldPlanetID should be nil in Phase 5")
	}
	if civ.HomeworldDescription == "" {
		t.Error("empty HomeworldDescription")
	}
	if fake.CompleteCalls != 0 {
		t.Errorf("Complete called %d times, want 0 (description is now structured)", fake.CompleteCalls)
	}
	if fake.CompleteJSONCalls != 3 {
		t.Errorf("CompleteJSON called %d times, want 3 (description, profile, name+flavor)", fake.CompleteJSONCalls)
	}
	if civ.TechProfile.DesignPhilosophy == "" {
		t.Error("empty TechProfile.DesignPhilosophy")
	}
}

func TestGenerateCivilization_ValidationRetrySucceeds(t *testing.T) {
	// First tech-profile response references a bogus mixture. Second is
	// valid. Expect 2 CompleteJSON calls for step 4 + 1 for step 5 = 3.
	bogus := `{
  "preferred_mixture_ids": ["NotARealMixture"],
  "preferred_cooling_methods": ["regenerative"],
  "preferred_ignition_types": ["spark"],
  "aversion_to_cryogenics": 0.2,
  "far_drive_family": "none",
  "tech_tier": 3
}`
	fake := &llm.FakeClient{
		CompleteJSONResponses: []string{validDescriptionJSON, bogus, validTechProfileJSON(t), validNameFlavorJSON},
	}

	civ, _, err := GenerateCivilization(context.Background(), fake, 42)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if civ == nil {
		t.Fatal("nil civ")
	}
	if fake.CompleteJSONCalls != 4 {
		t.Errorf("CompleteJSON called %d times, want 4 (description + step 4 retry + step 5)", fake.CompleteJSONCalls)
	}
}

func TestGenerateCivilization_ValidationHardFail(t *testing.T) {
	bogus := `{
  "preferred_mixture_ids": ["NotARealMixture"],
  "preferred_cooling_methods": ["spark"],
  "preferred_ignition_types": ["ablative"],
  "aversion_to_cryogenics": 1.5,
  "far_drive_family": "warp-bubble",
  "tech_tier": 9
}`
	fake := &llm.FakeClient{
		CompleteJSONResponses: []string{validDescriptionJSON, bogus, bogus},
	}

	_, _, err := GenerateCivilization(context.Background(), fake, 42)
	if err == nil {
		t.Fatal("expected error after two validation failures")
	}
	if !strings.Contains(err.Error(), "validation failed twice") {
		t.Errorf("error message should mention both validation failures: %v", err)
	}
}

func TestGenerateCivilization_DescriptionErrorBubbles(t *testing.T) {
	boom := errors.New("boom")
	fake := &llm.FakeClient{
		CompleteJSONErrs: []error{boom},
	}
	_, _, err := GenerateCivilization(context.Background(), fake, 42)
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got: %v", err)
	}
	if fake.CompleteJSONCalls != 1 {
		t.Errorf("only the description call should have fired; got %d", fake.CompleteJSONCalls)
	}
}

func TestSampleAgeYears_Deterministic(t *testing.T) {
	a := sampleAgeYears(42)
	b := sampleAgeYears(42)
	if a != b {
		t.Errorf("non-deterministic: %d vs %d", a, b)
	}
	if a < 100 || a > 10_000_000 {
		t.Errorf("age %d out of [100, 10_000_000]", a)
	}
}

func TestValidateTechProfile_CatchesEveryField(t *testing.T) {
	ids := sortedMixtureIDs()
	good := techProfileResponse{
		PreferredMixtureIDs:     []string{ids[0]},
		PreferredCoolingMethods: []string{"regenerative"},
		PreferredIgnitionTypes:  []string{"spark"},
		AversionToCryogenics:    0.5,
		FarDriveFamily:          "none",
		TechTier:                3,
	}
	if err := validateTechProfile(good); err != nil {
		t.Fatalf("good profile failed validation: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*techProfileResponse)
		needle string
	}{
		{"bad tier", func(r *techProfileResponse) { r.TechTier = 9 }, "tech_tier"},
		{"bad aversion", func(r *techProfileResponse) { r.AversionToCryogenics = 2.0 }, "aversion_to_cryogenics"},
		{"bogus mixture", func(r *techProfileResponse) { r.PreferredMixtureIDs = []string{"Zzz"} }, "does not resolve"},
		{"bad cooling", func(r *techProfileResponse) { r.PreferredCoolingMethods = []string{"spark"} }, "cooling method"},
		{"bad ignition", func(r *techProfileResponse) { r.PreferredIgnitionTypes = []string{"ablative"} }, "ignition method"},
		{"bad family", func(r *techProfileResponse) { r.FarDriveFamily = "warp" }, "far_drive_family"},
		{"empty mixtures", func(r *techProfileResponse) { r.PreferredMixtureIDs = nil }, "preferred_mixture_ids"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := good
			tc.mutate(&bad)
			err := validateTechProfile(bad)
			if err == nil {
				t.Fatalf("expected error mentioning %q", tc.needle)
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Errorf("error %q missing needle %q", err.Error(), tc.needle)
			}
		})
	}
}
