package id

import (
	"context"
	"fmt"
)

type Generator interface {
	MustGenerate(context.Context) string
}

func GetGenerator(name string) (Generator, error) {
	switch name {
	case "uuid":
		return NewUUIDGenerator(), nil
	default:
		return nil, fmt.Errorf("generator with name %q is not supported", name)
	}
}
