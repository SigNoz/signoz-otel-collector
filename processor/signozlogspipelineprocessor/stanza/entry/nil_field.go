package signozstanzaentry

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type NilField struct{}

func (NilField) Get(*entry.Entry) (any, bool) {
	return nil, true
}

func (NilField) Set(*entry.Entry, any) error {
	return nil
}

func (NilField) Delete(*entry.Entry) (any, bool) {
	return nil, true
}

func (NilField) String() string {
	return "$nil"
}

func NewNilField() entry.Field {
	return entry.Field{FieldInterface: NilField{}}
}
