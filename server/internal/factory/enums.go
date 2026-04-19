package factory

import "fmt"

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
