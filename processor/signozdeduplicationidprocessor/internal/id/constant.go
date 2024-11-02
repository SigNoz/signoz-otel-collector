package id

import (
	"context"
)

type constantGenerator struct {
	val string
}

func NewConstantGenerator(val string) Generator {
	return &constantGenerator{
		val: val,
	}
}

func (id *constantGenerator) MustGenerate(_ context.Context) string {
	return id.val
}
