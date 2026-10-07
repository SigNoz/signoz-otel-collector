package fieldvalues

import (
	"math"

	"github.com/cespare/xxhash/v2"
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

// fieldID identifies a field, its context and name, within one signal.
type fieldID uint64

func fieldIDOf(ctx fieldContext, name string) fieldID {
	return fieldID(mix64(xxhash.Sum64String(name) ^ uint64(ctx)*0x9e3779b97f4a7c15))
}

// pair is one `field = value` of a record. Bool values are kept in str as
// "true" or "false". The hashes are computed once, when the pair is made:
// field identifies the field, vh the value, and h the pair.
type pair struct {
	ctx   fieldContext
	typ   dataType
	name  string
	str   string
	num   float64
	field fieldID
	vh    uint64
	h     uint64
}

func newPair(ctx fieldContext, name string, typ dataType, str string, num float64) pair {
	p := pair{ctx: ctx, typ: typ, name: name, str: str, num: num, field: fieldIDOf(ctx, name)}
	if typ == typeNumber {
		p.vh = mix64(math.Float64bits(num) ^ uint64(typ)<<56)
	} else {
		p.vh = mix64(xxhash.Sum64String(str) ^ uint64(typ)<<56)
	}
	p.h = mix64(uint64(p.field) ^ p.vh*0xff51afd7ed558ccd)
	return p
}

func stringPair(ctx fieldContext, name, value string) pair {
	return newPair(ctx, name, typeString, value, 0)
}

func numberPair(ctx fieldContext, name string, value float64) pair {
	return newPair(ctx, name, typeNumber, "", value)
}

func boolPair(ctx fieldContext, name string, value bool) pair {
	if value {
		return newPair(ctx, name, typeBool, "true", 0)
	}
	return newPair(ctx, name, typeBool, "false", 0)
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

// setHash identifies a set, or a resource, by its pairs. It adds the pair
// hashes, so the order of the attributes does not change the id. The result
// is never 0 or 1, which mark resource rows and overflow sets.
func setHash(pairs []pair) uint64 {
	var sum uint64
	for i := range pairs {
		sum += pairs[i].h
	}
	h := mix64(sum ^ uint64(len(pairs)))
	if h <= overflowAttrsHash {
		h += 2
	}
	return h
}

func setKey(resourceHash, attrsHash uint64) uint64 {
	return mix64(resourceHash*0x9e3779b97f4a7c15 ^ attrsHash ^ 'S')
}

func pairKey(resourceHash, attrsHash uint64, p *pair) uint64 {
	return mix64(mix64(resourceHash*0x9e3779b97f4a7c15^attrsHash^'P') ^ p.h)
}

// nameHash hashes a metric name for resourceKey and labelKey. Logs and traces
// use the hash of the empty name.
func nameHash(metricName string) uint64 {
	return xxhash.Sum64String(metricName)
}

var emptyNameHash = nameHash("")

func resourceKey(metricNameHash, resourceHash uint64) uint64 {
	return mix64(metricNameHash*0x9e3779b97f4a7c15 ^ resourceHash ^ 'R')
}

func labelKey(metricNameHash uint64, p *pair) uint64 {
	return mix64(metricNameHash*0x9e3779b97f4a7c15 ^ uint64(fieldIDOf(p.ctx, p.name)) ^ 'K')
}

// mix64 is the splitmix64 finalizer. It spreads the bits of its input, so
// that any bit range of the result can pick a cache bucket.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}
