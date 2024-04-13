package strategies

import (
	"fmt"
)

// Strategy is an enum denoting the database strategy. Implements the pflag.Value interface.
//
//	type Value interface {
//		String() string
//		Set(string) error
//		Type() string
//	}
type Strategy struct {
	name string
}

var (
	Postgres = Strategy{name: "postgres"}
	Off      = Strategy{name: "off"}
)

func AllowedStrategies() []string {
	return []string{
		Postgres.name,
		Off.name,
	}
}

func NewStrategy(name string) (Strategy, error) {
	switch name {
	case "postgres":
		return Postgres, nil
	case "off":
		return Off, nil
	}

	return Strategy{}, fmt.Errorf("strategy %s not supported", name)
}

func (enum *Strategy) String() string {
	return enum.name
}

func (enum *Strategy) Set(name string) error {
	strategy, err := NewStrategy(name)
	if err != nil {
		return err
	}

	*enum = strategy
	return nil
}

func (enum *Strategy) Type() string {
	return "Strategy"
}
