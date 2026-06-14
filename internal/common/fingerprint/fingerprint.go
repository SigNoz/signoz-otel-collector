package fingerprint

import (
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
	attributes := make(Attributes, 0, attrs.Len()+len(extras))
	attrs.Range(func(k string, v pcommon.Value) bool {
		attributes = append(attributes, Attribute{Key: k, Value: Value{DataType: v.Type(), Val: v.AsString()}})
		return true
	})
	for k, v := range extras {
		attributes = append(attributes, Attribute{Key: k, Value: Value{DataType: pcommon.ValueTypeStr, Val: v}})
	}
	attributes.sortAndDedup()

	return &Fingerprint{
		attributes: attributes,
		hash:       hashAttributes(offset, attributes),
		typ:        typ,
	}
}

// hashAttributes hashes attributes in their stored (key-sorted) order, so the
// hash is independent of the source map's iteration order without sorting here.
func hashAttributes(offset uint64, attributes Attributes) uint64 {
	hash := offset
	for _, a := range attributes {
		hash = hashAdd(hash, a.Key)
		hash = hashAddByte(hash, separatorByte)
		hash = hashAdd(hash, a.Value.Val)
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
	// Fast path: when the drop set touches none of this fingerprint's keys the
	// reduced attribute set is identical, so the (read-only) attribute slice can
	// be shared and only the hash recomputed from the new offset. This is the
	// common case for the point fingerprint when the rule drops only resource-
	// or scope-level keys, and it avoids copying the slice on every datapoint.
	attributes := f.attributes
	dropsAny := false
	for _, a := range f.attributes {
		if drop(a.Key) {
			dropsAny = true
			break
		}
	}
	if dropsAny {
		// Filtering a sorted, de-duplicated slice keeps it sorted and
		// de-duplicated, so no re-sort is needed.
		attributes = make(Attributes, 0, len(f.attributes))
		for _, a := range f.attributes {
			if drop(a.Key) {
				continue
			}
			attributes = append(attributes, a)
		}
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
	for _, a := range f.attributes {
		attrMap[a.Key] = a.Value.Val
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
