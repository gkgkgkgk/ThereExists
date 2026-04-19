package factory

import (
	"math"
	"math/rand"
)

// LogUniform samples from [lo, hi] with a log-uniform distribution. Used
// when a range spans orders of magnitude (thrust, dry mass).
func LogUniform(lo, hi float64, rng *rand.Rand) float64 {
	if lo <= 0 || hi <= 0 {
		// Degenerate input — fall back to uniform. Validate() should
		// have caught this, but don't NaN at runtime.
		return lo + rng.Float64()*(hi-lo)
	}
	logLo := math.Log(lo)
	logHi := math.Log(hi)
	return math.Exp(logLo + rng.Float64()*(logHi-logLo))
}

// Uniform samples from [lo, hi] uniformly.
func Uniform(lo, hi float64, rng *rand.Rand) float64 {
	return lo + rng.Float64()*(hi-lo)
}

// Clamp01 clamps x to [0, 1].
func Clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
