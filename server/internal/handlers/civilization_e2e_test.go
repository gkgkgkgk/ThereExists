package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory/assembly"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
)

// TestCivgenEndToEnd exercises the full handler path with a fake LLM —
// proves routing, marshaling, and the pipeline agree on field names.
func TestCivgenEndToEnd(t *testing.T) {
	fake := &llm.FakeClient{
		CompleteJSONResponses: []string{validDescriptionJSON, validTechProfileJSON(t), validNameFlavorJSON},
	}
	h := NewCivilizationHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/civilizations/generate", h.Generate)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/civilizations/generate?seed=42", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d; body=%s", resp.StatusCode, body)
	}

	var out struct {
		Civilization map[string]any `json:"civilization"`
		Planet       map[string]any `json:"planet"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Civilization == nil || out.Planet == nil {
		t.Fatalf("missing top-level keys: %+v", out)
	}

	// Spot-check the civ. Civilization is marshaled with Go-default field
	// names (no json tags on the struct) so keys are capitalized.
	if tier, ok := out.Civilization["TechTier"].(float64); !ok || tier < 1 || tier > 5 {
		t.Errorf("TechTier missing or out of range: %v", out.Civilization["TechTier"])
	}
	if age, ok := out.Civilization["AgeYears"].(float64); !ok || age < 100 {
		t.Errorf("AgeYears missing or too small: %v", out.Civilization["AgeYears"])
	}

	// Planet uses json tags (snake_case).
	if ptype, _ := out.Planet["type"].(string); ptype == "" {
		t.Errorf("planet.type missing: %v", out.Planet)
	}
}

func TestCivgenFactoryVersion(t *testing.T) {
	if assembly.FactoryVersion != "phase5_1-v1" {
		t.Errorf("FactoryVersion = %q, want phase5_1-v1", assembly.FactoryVersion)
	}
}
