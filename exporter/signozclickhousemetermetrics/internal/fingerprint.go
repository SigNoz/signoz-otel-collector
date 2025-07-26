package internal

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

	return &Fingerprint{
		attributes: attributes,
		hash:       hash,
		typ:        typ,
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
