package fieldvalues

import (
	"math"
	"sort"
)

// fieldContext values match the Enum8 of the field_context column.
type fieldContext uint8

const (
	contextResource  fieldContext = 1
	contextAttribute fieldContext = 2
	contextScope     fieldContext = 3
	contextSpan      fieldContext = 4
	contextLog       fieldContext = 5
	contextEvent     fieldContext = 6
	contextBody      fieldContext = 7
)

func (c fieldContext) String() string {
	switch c {
	case contextResource:
		return "resource"
	case contextAttribute:
		return "attribute"
	case contextScope:
		return "scope"
	case contextSpan:
		return "span"
	case contextLog:
		return "log"
	case contextEvent:
		return "event"
	case contextBody:
		return "body"
	}
	return ""
}

func parseFieldContext(s string) (fieldContext, bool) {
	for c := contextResource; c <= contextBody; c++ {
		if c.String() == s {
			return c, true
		}
	}
	return 0, false
}

// dataType values match the Enum8 of the field_data_type column.
type dataType uint8

const (
	typeString dataType = 1
	typeNumber dataType = 2
	typeBool   dataType = 3
)

func (d dataType) String() string {
	switch d {
	case typeString:
		return "string"
	case typeNumber:
		return "number"
	case typeBool:
		return "bool"
	}
	return ""
}

type fieldKey struct {
	ctx  fieldContext
	name string
}

// pair is one `field = value` of a record. Bool values are kept in str as
// "true" or "false".
type pair struct {
	ctx  fieldContext
	name string
	typ  dataType
	str  string
	num  float64
}

func (p pair) key() fieldKey {
	return fieldKey{ctx: p.ctx, name: p.name}
}

func (p pair) valueHash() uint64 {
	h := hashByte(fnvOffset, byte(p.typ))
	if p.typ == typeNumber {
		h = hashUint64(h, math.Float64bits(p.num))
	} else {
		h = hashString(h, p.str)
	}
	return mix64(h)
}

func (p pair) hash() uint64 {
	h := hashByte(fnvOffset, byte(p.ctx))
	h = hashString(h, p.name)
	h = hashByte(h, separatorByte)
	return mix64(hashUint64(h, p.valueHash()))
}

const (
	// resourceAttrsHash is the attrs_hash of the rows of a resource.
	resourceAttrsHash uint64 = 0
	// overflowAttrsHash is the attrs_hash of the overflow set of a resource.
	overflowAttrsHash uint64 = 1
)

// row is one row of field_values_sets.
type row struct {
	metricName   string
	p            pair
	resourceHash uint64
	attrsHash    uint64
	inHash       bool
	seenMillis   uint64
}

// setHash identifies a set, or a resource, by its pairs. The pairs are sorted
// by context and name, so the order of the attributes does not change the id.
// The result is never 0 or 1, which mark resource rows and overflow sets.
func setHash(pairs []pair) uint64 {
	sorted := make([]pair, len(pairs))
	copy(sorted, pairs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ctx != sorted[j].ctx {
			return sorted[i].ctx < sorted[j].ctx
		}
		return sorted[i].name < sorted[j].name
	})
	h := fnvOffset
	for _, p := range sorted {
		h = hashUint64(h, p.hash())
	}
	h = mix64(h)
	if h <= overflowAttrsHash {
		h += 2
	}
	return h
}

func setKey(resourceHash, attrsHash uint64) uint64 {
	h := hashByte(fnvOffset, 'S')
	h = hashUint64(h, resourceHash)
	return mix64(hashUint64(h, attrsHash))
}

func pairKey(resourceHash, attrsHash uint64, p pair) uint64 {
	h := hashByte(fnvOffset, 'P')
	h = hashUint64(h, resourceHash)
	h = hashUint64(h, attrsHash)
	return mix64(hashUint64(h, p.hash()))
}

func resourceKey(metricName string, resourceHash uint64) uint64 {
	h := hashByte(fnvOffset, 'R')
	h = hashString(h, metricName)
	h = hashByte(h, separatorByte)
	return mix64(hashUint64(h, resourceHash))
}

const (
	fnvOffset     uint64 = 14695981039346656037
	fnvPrime      uint64 = 1099511628211
	separatorByte byte   = 255
)

func hashString(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func hashByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	h *= fnvPrime
	return h
}

func hashUint64(h, v uint64) uint64 {
	for i := 0; i < 8; i++ {
		h ^= v & 0xff
		h *= fnvPrime
		v >>= 8
	}
	return h
}

// mix64 is the splitmix64 finalizer. It spreads the bits of an FNV hash, so
// that any bit range of the result can pick a cache bucket.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}
