package flight_test

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/content"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
)

// External test package — the flight tests reference content's
// archetype catalog (content.RCAStandard etc.), so the tests live in
// package flight_test to avoid the content → flight → test cycle that
// an internal test package would create.

// rollOne rolls an engine from RCAStandard with the given seed under
// the nil-civ baseline (tier 3, generic shipwright stamp).
func rollOne(t *testing.T, seed int64) *flight.LiquidChemicalEngine {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	e, err := flight.GenerateLiquidChemicalEngine(content.RCAStandard, nil, rng)
	if err != nil {
		t.Fatalf("GenerateLiquidChemicalEngine: %v", err)
	}
	return e
}

// TestDeterminism — same seed produces identical engines across runs.
// UUIDs break bit-equality, so compare every other field via reflection
// after zeroing ID.
func TestDeterminism(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		a := rollOne(t, seed)
		b := rollOne(t, seed)
		a.ID, b.ID = [16]byte{}, [16]byte{}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("seed %d: engines differ across runs", seed)
		}
	}
}

// TestDAGInvariants — sweep many seeds and assert every generated engine
// respects the DAG's structural guarantees.
func TestDAGInvariants(t *testing.T) {
	for seed := int64(0); seed < 1000; seed++ {
		e := rollOne(t, seed)

		if e.IspAtRefPressureSec > e.IspVacuumSec {
			t.Fatalf("seed %d: atmospheric Isp (%v) > vacuum Isp (%v)", seed, e.IspAtRefPressureSec, e.IspVacuumSec)
		}
		if e.CanThrottle != (e.MaxThrottle > e.MinThrottle) {
			t.Fatalf("seed %d: CanThrottle=%v but min=%v max=%v", seed, e.CanThrottle, e.MinThrottle, e.MaxThrottle)
		}
		if (e.GimbalRangeDegrees == 0) != (e.DryMassKg < content.RCAStandard.GimbalEligibleMassKg) {
			t.Fatalf("seed %d: gimbal gating violated — mass=%v eligible=%v gimbal=%v",
				seed, e.DryMassKg, content.RCAStandard.GimbalEligibleMassKg, e.GimbalRangeDegrees)
		}
		if (e.InitialAblatorMassKg > 0) != (e.CoolingMethod == factory.Ablative) {
			t.Fatalf("seed %d: ablator mass (%v) vs cooling (%v) mismatch", seed, e.InitialAblatorMassKg, e.CoolingMethod)
		}
		if e.Count < 1 {
			t.Fatalf("seed %d: Count must be >= 1, got %d", seed, e.Count)
		}
		if len(e.Health) != e.Count {
			t.Fatalf("seed %d: len(Health)=%d != Count=%d", seed, len(e.Health), e.Count)
		}
		for i, h := range e.Health {
			if h < 0 || h > 1 {
				t.Fatalf("seed %d: Health[%d] out of [0,1]: %v", seed, i, h)
			}
		}
		if e.RestartsUsed != 0 || e.IsFiring {
			t.Fatalf("seed %d: non-zero runtime state at generation", seed)
		}
		if e.AblatorMassRemainingKg != e.InitialAblatorMassKg {
			t.Fatalf("seed %d: ablator remaining (%v) != initial (%v)", seed, e.AblatorMassRemainingKg, e.InitialAblatorMassKg)
		}
	}
}

// TestReachability — pin an explicit set of reachable enum values so a
// future archetype tweak that silently drops a cooling/ignition option
// trips the test. Hardcoded, not runtime-derived, so the test is a
// tripwire for distribution shifts, not a self-reflecting tautology.
func TestReachability(t *testing.T) {
	// RCAStandard: chamber pressure 5–25 bar. Both Ablative
	// (<=150 bar) and Radiative (<=40 bar) should survive the pressure
	// filter on every roll. Film is unconditional. So all three are
	// reachable.
	expectedCooling := map[factory.CoolingMethod]bool{
		factory.Ablative:  false,
		factory.Radiative: false,
		factory.Film:      false,
	}
	// RCAStandard.AllowedMixtures = {MMH_NTO, Hydrazine}.
	// MMH_NTO is hypergolic → Hypergolic. Hydrazine is monopropellant
	// (default Config=0) → Catalytic. Spark/Pyrotechnic are not
	// reachable here.
	expectedIgnition := map[factory.IgnitionMethod]bool{
		factory.Hypergolic: false,
		factory.Catalytic:  false,
	}

	for seed := int64(0); seed < 10_000; seed++ {
		e := rollOne(t, seed)
		if _, ok := expectedCooling[e.CoolingMethod]; ok {
			expectedCooling[e.CoolingMethod] = true
		} else {
			t.Fatalf("seed %d: unexpected cooling method %v for RCAStandard", seed, e.CoolingMethod)
		}
		if _, ok := expectedIgnition[e.IgnitionMethod]; ok {
			expectedIgnition[e.IgnitionMethod] = true
		} else {
			t.Fatalf("seed %d: unexpected ignition method %v for RCAStandard", seed, e.IgnitionMethod)
		}
	}
	for m, hit := range expectedCooling {
		if !hit {
			t.Errorf("cooling method %v never reached across 10000 seeds", m)
		}
	}
	for m, hit := range expectedIgnition {
		if !hit {
			t.Errorf("ignition method %v never reached across 10000 seeds", m)
		}
	}
}

// TestIspAt_Monotonic — Isp should decrease monotonically as ambient
// pressure rises from vacuum up to (and past) the reference pressure.
func TestIspAt_Monotonic(t *testing.T) {
	e := rollOne(t, 123)
	prev := e.IspAt(0)
	if prev != e.IspVacuumSec {
		t.Fatalf("IspAt(0) = %v, expected IspVacuumSec = %v", prev, e.IspVacuumSec)
	}
	for pa := 10_000.0; pa <= 200_000.0; pa += 10_000 {
		cur := e.IspAt(pa)
		if cur > prev+1e-9 {
			t.Fatalf("Isp increased with pressure: %v Pa: prev=%v cur=%v", pa, prev, cur)
		}
		prev = cur
	}
	if e.IspAt(1_000_000) != 0 {
		t.Fatal("IspAt far past reference should floor at 0")
	}
}

// TestHasRestartsRemaining — sentinel semantics for MaxRestarts == -1.
func TestHasRestartsRemaining(t *testing.T) {
	e := &flight.LiquidChemicalEngine{}
	e.MaxRestarts = -1
	e.RestartsUsed = 1_000_000
	if !e.HasRestartsRemaining() {
		t.Fatal("MaxRestarts=-1 means unlimited; RestartsUsed should be irrelevant")
	}
	e.MaxRestarts = 3
	e.RestartsUsed = 3
	if e.HasRestartsRemaining() {
		t.Fatal("at cap: HasRestartsRemaining should be false")
	}
	e.RestartsUsed = 2
	if !e.HasRestartsRemaining() {
		t.Fatal("below cap: HasRestartsRemaining should be true")
	}
}
