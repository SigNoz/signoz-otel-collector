// brought in as-is from opentelemetry-collector-contrib
package signozstanzaentry

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// NilField is a struct that implements Field, but
// does nothing for all its operations. It is useful
// as a default no-op field to avoid nil checks.
type NilField struct{}

// Get will return always return nil
func (NilField) Get(*entry.Entry) (any, bool) {
	return nil, true
}

// Set will do nothing and return no error
func (NilField) Set(*entry.Entry, any) error {
	return nil
}

// Delete will do nothing and return no error
func (NilField) Delete(*entry.Entry) (any, bool) {
	return nil, true
}

func (NilField) String() string {
	return "$nil"
}

// NewNilField will create a new nil field
func NewNilField() entry.Field {
	return entry.Field{FieldInterface: NilField{}}
}
