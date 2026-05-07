package factory

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/google/uuid"
)

// PlanetType is the coarse taxonomy used by the Phase 5 planet stub. Full
// planet generation replaces this with a richer classification; the stub
// is deliberately small (6 values) so the civ-generation LLM prompt can
// enumerate it trivially.
type PlanetType int

const (
	Terrestrial PlanetType = iota
	OceanWorld
	IceWorld
	DesertWorld
	GasGiant
	LavaWorld
)

func (p PlanetType) String() string {
	switch p {
	case Terrestrial:
		return "terrestrial"
	case OceanWorld:
		return "ocean_world"
	case IceWorld:
		return "ice_world"
	case DesertWorld:
		return "desert_world"
	case GasGiant:
		return "gas_giant"
	case LavaWorld:
		return "lava_world"
	}
	return fmt.Sprintf("planet_type(%d)", int(p))
}

func (p PlanetType) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

func (p *PlanetType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "terrestrial":
		*p = Terrestrial
	case "ocean_world":
		*p = OceanWorld
	case "ice_world":
		*p = IceWorld
	case "desert_world":
		*p = DesertWorld
	case "gas_giant":
		*p = GasGiant
	case "lava_world":
		*p = LavaWorld
	default:
		return fmt.Errorf("unknown planet type: %q", string(text))
	}
	return nil
}

// Planet is the Phase 5 stub — just enough to seed civ generation. No
// biomes, moons, resource deposits, or orbital mechanics. Full planet
// generation is a later phase; the civ pipeline depends only on this
// struct's shape, so the generator can be swapped without touching
// civgen.
type Planet struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Type                  PlanetType `json:"type"`
	SurfaceGravityG       float64    `json:"surface_gravity_g"`
	AtmospherePressureAtm float64    `json:"atmosphere_pressure_atm"`
	AtmosphereComposition []string   `json:"atmosphere_composition"`
	SurfaceTempKRange     [2]float64 `json:"surface_temp_k_range"`
	HasLiquidWater        bool       `json:"has_liquid_water"`
	HasMagnetosphere      bool       `json:"has_magnetosphere"`
	StarType              string     `json:"star_type"`
	OrbitalPeriodDays     float64    `json:"orbital_period_days"`
}

// planetTypeParams holds per-type sampling ranges. Kept internally
// consistent (e.g. gas giants can't have liquid water; lava worlds
// almost never have surviving magnetospheres) by encoding constraints in
// per-type probabilities rather than post-hoc validation.
type planetTypeParams struct {
	GravityG            [2]float64
	PressureAtm         [2]float64
	TempK               [2]float64
	WaterProb           float64
	MagnetoProb         float64
	AtmosphereOptions   []string
	AtmosphereSampleMin int
	AtmosphereSampleMax int
}

var planetParams = map[PlanetType]planetTypeParams{
	Terrestrial: {
		GravityG:            [2]float64{0.3, 2.0},
		PressureAtm:         [2]float64{0.2, 3.0},
		TempK:               [2]float64{200, 350},
		WaterProb:           0.8,
		MagnetoProb:         0.7,
		AtmosphereOptions:   []string{"N2", "O2", "CO2", "Ar", "H2O vapor", "trace CH4"},
		AtmosphereSampleMin: 2,
		AtmosphereSampleMax: 4,
	},
	OceanWorld: {
		GravityG:            [2]float64{0.6, 1.8},
		PressureAtm:         [2]float64{0.5, 5.0},
		TempK:               [2]float64{260, 320},
		WaterProb:           1.0,
		MagnetoProb:         0.5,
		AtmosphereOptions:   []string{"N2", "H2O vapor", "O2", "CO2", "CH4"},
		AtmosphereSampleMin: 2,
		AtmosphereSampleMax: 3,
	},
	IceWorld: {
		GravityG:            [2]float64{0.2, 1.2},
		PressureAtm:         [2]float64{0.0, 0.5},
		TempK:               [2]float64{40, 180},
		WaterProb:           0.2,
		MagnetoProb:         0.3,
		AtmosphereOptions:   []string{"N2", "CH4", "CO2", "H2O vapor", "trace Ar", "thin He"},
		AtmosphereSampleMin: 1,
		AtmosphereSampleMax: 3,
	},
	DesertWorld: {
		GravityG:            [2]float64{0.3, 1.5},
		PressureAtm:         [2]float64{0.1, 2.0},
		TempK:               [2]float64{220, 400},
		WaterProb:           0.05,
		MagnetoProb:         0.4,
		AtmosphereOptions:   []string{"CO2", "N2", "SO2", "dust aerosols", "trace Ar"},
		AtmosphereSampleMin: 2,
		AtmosphereSampleMax: 4,
	},
	GasGiant: {
		GravityG:            [2]float64{1.5, 5.0},
		PressureAtm:         [2]float64{50, 200},
		TempK:               [2]float64{50, 200},
		WaterProb:           0.0,
		MagnetoProb:         0.95,
		AtmosphereOptions:   []string{"H2", "He", "CH4", "NH3", "H2O clouds"},
		AtmosphereSampleMin: 2,
		AtmosphereSampleMax: 4,
	},
	LavaWorld: {
		GravityG:            [2]float64{0.3, 2.5},
		PressureAtm:         [2]float64{0.0, 3.0},
		TempK:               [2]float64{700, 1800},
		WaterProb:           0.0,
		MagnetoProb:         0.1,
		AtmosphereOptions:   []string{"SO2", "silicate vapors", "CO2", "Na", "K"},
		AtmosphereSampleMin: 1,
		AtmosphereSampleMax: 3,
	},
}

var starTypes = []string{"G-type", "K-type", "M-dwarf", "F-type", "binary K+M"}

// Name-generation tables. Small syllable set, deterministic per seed.
var (
	planetNamePrefixes = []string{"Kael", "Tor", "Vex", "Zan", "Mir", "Aur", "Nyx", "Sol", "Dra", "Orb", "Hel", "Cry", "Pho", "Thes", "Xen"}
	planetNameSuffixes = []string{"os", "ara", "eth", "ion", "us", "ix", "al", "orr", "ane", "is", "ax", "oth"}
)

// GeneratePlanet rolls a plausible planet procedurally. Deterministic per
// seed except for ID (uuid.NewString is non-reproducible); that's fine
// for Phase 5 since planets don't persist. When the full planet-gen
// phase lands, this function is replaced and civgen is unaffected.
func GeneratePlanet(seed int64) (*Planet, error) {
	r := rand.New(rand.NewSource(seed))

	pt := PlanetType(r.Intn(6))
	params := planetParams[pt]

	temp0 := randRange(r, params.TempK[0], params.TempK[1])
	temp1 := temp0 + r.Float64()*(params.TempK[1]-temp0)

	p := &Planet{
		ID:                    uuid.NewString(),
		Name:                  rollPlanetName(r),
		Type:                  pt,
		SurfaceGravityG:       randRange(r, params.GravityG[0], params.GravityG[1]),
		AtmospherePressureAtm: randRange(r, params.PressureAtm[0], params.PressureAtm[1]),
		AtmosphereComposition: sampleAtmosphere(r, params),
		SurfaceTempKRange:     [2]float64{temp0, temp1},
		HasLiquidWater:        r.Float64() < params.WaterProb,
		HasMagnetosphere:      r.Float64() < params.MagnetoProb,
		StarType:              starTypes[r.Intn(len(starTypes))],
		OrbitalPeriodDays:     randRange(r, 30, 100000),
	}
	return p, nil
}

// Describe renders the planet as a compact multi-line string suitable
// for inlining into an LLM prompt.
func (p *Planet) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", p.Name)
	fmt.Fprintf(&b, "Type: %s\n", p.Type)
	fmt.Fprintf(&b, "Surface gravity: %.2f g\n", p.SurfaceGravityG)
	fmt.Fprintf(&b, "Atmosphere pressure: %.2f atm\n", p.AtmospherePressureAtm)
	fmt.Fprintf(&b, "Atmosphere composition: %s\n", strings.Join(p.AtmosphereComposition, ", "))
	fmt.Fprintf(&b, "Surface temperature: %.0f–%.0f K\n", p.SurfaceTempKRange[0], p.SurfaceTempKRange[1])
	fmt.Fprintf(&b, "Liquid water: %v\n", p.HasLiquidWater)
	fmt.Fprintf(&b, "Magnetosphere: %v\n", p.HasMagnetosphere)
	fmt.Fprintf(&b, "Star: %s\n", p.StarType)
	fmt.Fprintf(&b, "Orbital period: %.0f days\n", p.OrbitalPeriodDays)
	return b.String()
}

func randRange(r *rand.Rand, lo, hi float64) float64 {
	if hi <= lo {
		return lo
	}
	return lo + r.Float64()*(hi-lo)
}

func rollPlanetName(r *rand.Rand) string {
	prefix := planetNamePrefixes[r.Intn(len(planetNamePrefixes))]
	suffix := planetNameSuffixes[r.Intn(len(planetNameSuffixes))]
	n := r.Intn(900) + 100
	// 40% of names get a numeric suffix (like real exoplanet catalog names).
	if r.Float64() < 0.4 {
		return fmt.Sprintf("%s%s-%d", prefix, suffix, n)
	}
	return prefix + suffix
}

func sampleAtmosphere(r *rand.Rand, params planetTypeParams) []string {
	n := params.AtmosphereSampleMin
	if params.AtmosphereSampleMax > params.AtmosphereSampleMin {
		n += r.Intn(params.AtmosphereSampleMax - params.AtmosphereSampleMin + 1)
	}
	if n > len(params.AtmosphereOptions) {
		n = len(params.AtmosphereOptions)
	}
	// Partial Fisher-Yates: shuffle first n entries of a copy.
	opts := make([]string, len(params.AtmosphereOptions))
	copy(opts, params.AtmosphereOptions)
	for i := 0; i < n; i++ {
		j := i + r.Intn(len(opts)-i)
		opts[i], opts[j] = opts[j], opts[i]
	}
	return opts[:n]
}
