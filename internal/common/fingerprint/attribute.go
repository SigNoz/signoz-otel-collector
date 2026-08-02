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

// Attributes is a list of attributes kept sorted by Key and de-duplicated, so
// hashing and label rendering never have to sort.
type Attributes []Attribute

// sortAndDedup sorts in place by Key and drops duplicate keys, keeping the last
// occurrence. Since extras are appended last, an extra colliding with a real
// attribute wins, which keeps fingerprints stable.
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
