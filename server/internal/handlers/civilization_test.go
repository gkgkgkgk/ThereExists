package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// firstMixtureID returns a registry mixture so the test can build a
// valid tech-profile response without hardcoding IDs.
func firstMixtureID(t *testing.T) string {
	t.Helper()
	for id := range factory.Mixtures {
		return id
	}
	t.Fatal("no mixtures registered")
	return ""
}

func validTechProfileJSON(t *testing.T) string {
	t.Helper()
	id := firstMixtureID(t)
	return `{
  "preferred_mixture_ids": ["` + id + `"],
  "preferred_cooling_methods": ["regenerative"],
  "preferred_ignition_types": ["spark"],
  "aversion_to_cryogenics": 0.2,
  "far_drive_family": "none",
  "tech_tier": 3,
  "risk_tolerance": 0.4,
  "thrust_vs_isp_preference": 0.1
}`
}

const validNameFlavorJSON = `{"name":"Test Civ","flavor":"A testy civilization."}`

const validDescriptionJSON = `{"description":"They live in stone and think in long cycles.","design_philosophy":"austere pragmatism; overbuilt margins"}`

func TestCivilizationHandler_Success(t *testing.T) {
	fake := &llm.FakeClient{
		CompleteJSONResponses: []string{validDescriptionJSON, validTechProfileJSON(t), validNameFlavorJSON},
	}
	h := NewCivilizationHandler(fake)

	req := httptest.NewRequest("POST", "/api/civilizations/generate?seed=7", nil)
	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Civilization map[string]any `json:"civilization"`
		Planet       map[string]any `json:"planet"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, rr.Body.String())
	}
	if out.Civilization == nil {
		t.Error("missing civilization in response")
	}
	if out.Planet == nil {
		t.Error("missing planet in response")
	}
	if out.Civilization["Name"] == "" && out.Civilization["name"] == "" {
		t.Error("civ has empty name")
	}
}

func TestCivilizationHandler_InvalidSeed(t *testing.T) {
	h := NewCivilizationHandler(&llm.FakeClient{})
	req := httptest.NewRequest("POST", "/api/civilizations/generate?seed=notanumber", nil)
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestCivilizationHandler_LLMError(t *testing.T) {
	fake := &llm.FakeClient{CompleteJSONErrs: []error{errors.New("upstream down")}}
	h := NewCivilizationHandler(fake)
	req := httptest.NewRequest("POST", "/api/civilizations/generate?seed=7", nil)
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestCivilizationHandler_MissingKey(t *testing.T) {
	h := NewCivilizationHandler(nil)
	req := httptest.NewRequest("POST", "/api/civilizations/generate", nil)
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "OPENAI_API_KEY") {
		t.Errorf("503 body should mention the missing key: %s", rr.Body.String())
	}
}
