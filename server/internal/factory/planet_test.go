package factory

import "testing"

func TestGeneratePlanet_DeterministicExceptID(t *testing.T) {
	a, err := GeneratePlanet(42)
	if err != nil {
		t.Fatalf("GeneratePlanet: %v", err)
	}
	b, err := GeneratePlanet(42)
	if err != nil {
		t.Fatalf("GeneratePlanet: %v", err)
	}
	// ID is a fresh UUID per call — expected to differ. Everything else
	// is seeded and must match bit-for-bit.
	if a.ID == b.ID {
		t.Errorf("IDs unexpectedly matched — UUID should be fresh per call")
	}
	a.ID, b.ID = "", ""
	if a.Name != b.Name || a.Type != b.Type ||
		a.SurfaceGravityG != b.SurfaceGravityG ||
		a.AtmospherePressureAtm != b.AtmospherePressureAtm ||
		a.SurfaceTempKRange != b.SurfaceTempKRange ||
		a.HasLiquidWater != b.HasLiquidWater ||
		a.HasMagnetosphere != b.HasMagnetosphere ||
		a.StarType != b.StarType ||
		a.OrbitalPeriodDays != b.OrbitalPeriodDays {
		t.Errorf("scalar field mismatch:\n a=%+v\n b=%+v", a, b)
	}
	if len(a.AtmosphereComposition) != len(b.AtmosphereComposition) {
		t.Fatalf("atmosphere length mismatch: %d vs %d", len(a.AtmosphereComposition), len(b.AtmosphereComposition))
	}
	for i := range a.AtmosphereComposition {
		if a.AtmosphereComposition[i] != b.AtmosphereComposition[i] {
			t.Errorf("atmosphere[%d]: %q vs %q", i, a.AtmosphereComposition[i], b.AtmosphereComposition[i])
		}
	}
}

func TestGeneratePlanet_FieldRangeSweep(t *testing.T) {
	for s := range 1000 {
		seed := int64(s)
		p, err := GeneratePlanet(seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if p.Name == "" {
			t.Errorf("seed %d: empty name", seed)
		}
		if p.ID == "" {
			t.Errorf("seed %d: empty id", seed)
		}
		if int(p.Type) < 0 || int(p.Type) > 5 {
			t.Errorf("seed %d: type out of range: %d", seed, p.Type)
		}
		if p.SurfaceGravityG <= 0 {
			t.Errorf("seed %d: non-positive gravity: %f", seed, p.SurfaceGravityG)
		}
		if p.AtmospherePressureAtm < 0 {
			t.Errorf("seed %d: negative pressure: %f", seed, p.AtmospherePressureAtm)
		}
		if p.SurfaceTempKRange[0] > p.SurfaceTempKRange[1] {
			t.Errorf("seed %d: temp range inverted: %v", seed, p.SurfaceTempKRange)
		}
		if len(p.AtmosphereComposition) == 0 {
			t.Errorf("seed %d: empty atmosphere composition", seed)
		}
		if p.OrbitalPeriodDays <= 0 {
			t.Errorf("seed %d: non-positive orbital period: %f", seed, p.OrbitalPeriodDays)
		}
		if p.StarType == "" {
			t.Errorf("seed %d: empty star type", seed)
		}
	}
}

func TestPlanet_DescribeContainsKeyFields(t *testing.T) {
	p, err := GeneratePlanet(7)
	if err != nil {
		t.Fatalf("GeneratePlanet: %v", err)
	}
	desc := p.Describe()
	for _, needle := range []string{p.Name, p.Type.String(), p.StarType} {
		if !containsString(desc, needle) {
			t.Errorf("Describe() missing %q:\n%s", needle, desc)
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
