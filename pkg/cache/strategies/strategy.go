package strategies

import (
	"fmt"
)

// Strategy is an enum denoting the cache strategy. Implements the pflag.Value interface.
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
	Redis = Strategy{name: "redis"}
	Off   = Strategy{name: "off"}
)

func AllowedStrategies() []string {
	return []string{
		Redis.name,
		Off.name,
	}
}

func NewStrategy(name string) (Strategy, error) {
	switch name {
	case "redis":
		return Redis, nil
	case "off":
		return Off, nil
	default:
		return Strategy{}, fmt.Errorf("strategy %s not supported", name)
	}
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
