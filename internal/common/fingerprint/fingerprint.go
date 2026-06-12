package fingerprint

import (
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type FingerprintType struct{ s string }

func (t FingerprintType) String() string { return t.s }

var (
	ResourceFingerprintType FingerprintType = FingerprintType{s: "resource"}
	ScopeFingerprintType    FingerprintType = FingerprintType{s: "scope"}
	PointFingerprintType    FingerprintType = FingerprintType{s: "point"}
)

type Fingerprint struct {
	attributes Attributes
	hash       uint64
	typ        FingerprintType
}

func NewFingerprint(typ FingerprintType, offset uint64, attrs pcommon.Map, extras map[string]string) *Fingerprint {
	attributes := NewAttributesFromPcommonMap(attrs)

	for k, v := range extras {
		attributes[k] = Value{
			DataType: pcommon.ValueTypeStr,
			Val:      v,
		}
	}

	return &Fingerprint{
		attributes: attributes,
		hash:       hashAttributes(offset, attributes),
		typ:        typ,
	}
}

func hashAttributes(offset uint64, attributes Attributes) uint64 {
	hash := offset

	sortedKeys := make([]string, 0, len(attributes))
	for k := range attributes {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		v := attributes[k]
		hash = hashAdd(hash, k)
		hash = hashAddByte(hash, separatorByte)
		hash = hashAdd(hash, v.Val)
		hash = hashAddByte(hash, separatorByte)
	}

	return hash
}

// Reduced returns a new fingerprint computed over this fingerprint's
// attributes minus the keys for which drop returns true, hashed from the
// given offset. Reducing a resource -> scope -> point chain requires passing
// the previous reduced fingerprint's hash as the offset at each level, the
// same way the raw chain is built; series that differ only in dropped keys
// then collapse to the same reduced fingerprint.
func (f *Fingerprint) Reduced(offset uint64, drop func(key string) bool) *Fingerprint {
	attributes := make(Attributes, len(f.attributes))
	for k, v := range f.attributes {
		if drop(k) {
			continue
		}
		attributes[k] = v
	}

	return &Fingerprint{
		attributes: attributes,
		hash:       hashAttributes(offset, attributes),
		typ:        f.typ,
	}
}

func (f *Fingerprint) Attributes() Attributes {
	return f.attributes
}

func (f *Fingerprint) AttributesAsMap() map[string]string {
	attrMap := make(map[string]string, len(f.attributes))
	for k, v := range f.attributes {
		attrMap[k] = v.Val
	}
	return attrMap
}

func (f *Fingerprint) Hash() uint64 {
	return f.hash
}

func (f *Fingerprint) HashWithName(name string) uint64 {
	sum := f.hash
	sum = hashAdd(sum, "__name__")
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, name)
	return sum
}

func (f *Fingerprint) Type() FingerprintType {
	return f.typ
}
