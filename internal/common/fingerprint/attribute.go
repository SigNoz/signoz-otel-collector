package fingerprint

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Value struct {
	DataType pcommon.ValueType
	Val      string
}

// Attribute is a single key/value pair.
type Attribute struct {
	Key   string
	Value Value
}

// Attributes is a list of attributes kept sorted by Key and de-duplicated. A
// fingerprint is built once per datapoint on the export hot path; storing the
// attributes pre-sorted (instead of in a map) means hashing and label rendering
// never have to sort, and it replaces a per-datapoint map allocation with a
// single slice.
type Attributes []Attribute

// sortAndDedup sorts in place by Key and drops duplicate keys, keeping the last
// occurrence of each. Builders append the pcommon attributes first and the
// extras last, so on the (pathological) collision of an extra with a real
// attribute the extra wins — matching the previous map-based "extra overwrites
// attribute" behavior, and keeping fingerprints stable.
func (a *Attributes) sortAndDedup() {
	s := *a
	if len(s) < 2 {
		return
	}
	slices.SortStableFunc(s, func(x, y Attribute) int {
		switch {
		case x.Key < y.Key:
			return -1
		case x.Key > y.Key:
			return 1
		default:
			return 0
		}
	})
	w := 0
	for r := 0; r < len(s); r++ {
		if r+1 < len(s) && s[r].Key == s[r+1].Key {
			continue // earlier duplicate; the later occurrence (extra) wins
		}
		s[w] = s[r]
		w++
	}
	*a = s[:w]
}
