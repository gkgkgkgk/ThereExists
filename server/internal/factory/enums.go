package factory

import (
	"fmt"
	"strings"
)

// Enums shared across multiple system categories live here. Category-local
// enums (e.g. FlightSlot) live alongside the category itself.
//
// Current residents are all flight-engine configuration enums, but they're
// referenced by TechProfile in civilizations.go — so they must sit above
// any category subpackage to keep the factory import graph acyclic. When
// a non-flight category arrives, group its enums in a new section below.

// ───────────────────────── Flight / propulsion ─────────────────────────

// PropellantConfig describes how many propellant streams an engine burns.
type PropellantConfig int

const (
	Monopropellant PropellantConfig = iota
	Bipropellant
)

func (p PropellantConfig) String() string {
	switch p {
	case Monopropellant:
		return "monopropellant"
	case Bipropellant:
		return "bipropellant"
	}
	return fmt.Sprintf("propellant_config(%d)", int(p))
}

func (p PropellantConfig) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

// IgnitionMethod describes how an engine lights its propellant mixture.
// Derived from the rolled mixture's flags at generation time (Plan §2
// Group 7) — never declared independently on an archetype.
type IgnitionMethod int

const (
	Spark IgnitionMethod = iota
	Pyrotechnic
	Hypergolic
	Catalytic
)

func (i IgnitionMethod) String() string {
	switch i {
	case Spark:
		return "spark"
	case Pyrotechnic:
		return "pyrotechnic"
	case Hypergolic:
		return "hypergolic"
	case Catalytic:
		return "catalytic"
	}
	return fmt.Sprintf("ignition_method(%d)", int(i))
}

func (i IgnitionMethod) MarshalText() ([]byte, error) { return []byte(i.String()), nil }

func (i *IgnitionMethod) UnmarshalText(text []byte) error {
	v, ok := ParseIgnitionMethod(string(text))
	if !ok {
		return fmt.Errorf("unknown ignition method: %q", string(text))
	}
	*i = v
	return nil
}

// CoolingMethod describes how an engine rejects chamber heat. Each method
// has real teeth (Plan §2 "Cooling methods — each has real teeth"):
// Ablative depletes mass; Radiative caps throttle; Film penalises Isp;
// Regenerative is near-free but fails catastrophically out of envelope.
type CoolingMethod int

const (
	Ablative CoolingMethod = iota
	Regenerative
	Radiative
	Film
)

func (c CoolingMethod) String() string {
	switch c {
	case Ablative:
		return "ablative"
	case Regenerative:
		return "regenerative"
	case Radiative:
		return "radiative"
	case Film:
		return "film"
	}
	return fmt.Sprintf("cooling_method(%d)", int(c))
}

func (c CoolingMethod) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

func (c *CoolingMethod) UnmarshalText(text []byte) error {
	v, ok := ParseCoolingMethod(string(text))
	if !ok {
		return fmt.Errorf("unknown cooling method: %q", string(text))
	}
	*c = v
	return nil
}

// AllCoolingMethods enumerates every CoolingMethod value. Used by civgen
// to build the constrained-choice option menu and by ParseCoolingMethod.
var AllCoolingMethods = []CoolingMethod{Ablative, Regenerative, Radiative, Film}

// AllIgnitionMethods enumerates every IgnitionMethod value.
var AllIgnitionMethods = []IgnitionMethod{Spark, Pyrotechnic, Hypergolic, Catalytic}

// ParseCoolingMethod maps the String() output back to the enum. Case-
// insensitive to tolerate LLM casing drift.
func ParseCoolingMethod(s string) (CoolingMethod, bool) {
	for _, c := range AllCoolingMethods {
		if strings.EqualFold(c.String(), s) {
			return c, true
		}
	}
	return 0, false
}

// ParseIgnitionMethod maps the String() output back to the enum.
func ParseIgnitionMethod(s string) (IgnitionMethod, bool) {
	for _, i := range AllIgnitionMethods {
		if strings.EqualFold(i.String(), s) {
			return i, true
		}
	}
	return 0, false
}
