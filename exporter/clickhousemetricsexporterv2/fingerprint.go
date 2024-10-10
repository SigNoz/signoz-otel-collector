package clickhousemetricsexporterv2

import "go.opentelemetry.io/collector/pdata/pcommon"

const (
	offset64      uint64 = 14695981039346656037
	prime64       uint64 = 1099511628211
	separatorByte byte   = 255
)

// hashAdd adds a string to a fnv64a hash value, returning the updated hash.
func hashAdd(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// hashAddByte adds a byte to a fnv64a hash value, returning the updated hash.
func hashAddByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	h *= prime64
	return h
}

func Fingerprint(attrs pcommon.Map, scopeAttrs pcommon.Map, resAttrs pcommon.Map, name string) uint64 {
	if attrs.Len() == 0 && resAttrs.Len() == 0 && scopeAttrs.Len() == 0 {
		return offset64
	}

	sum := offset64

	resAttrs.Range(func(k string, v pcommon.Value) bool {
		sum = hashAdd(sum, k)
		sum = hashAddByte(sum, separatorByte)
		sum = hashAdd(sum, v.AsString())
		sum = hashAddByte(sum, separatorByte)
		return true
	})

	scopeAttrs.Range(func(k string, v pcommon.Value) bool {
		sum = hashAdd(sum, k)
		sum = hashAddByte(sum, separatorByte)
		sum = hashAdd(sum, v.AsString())
		sum = hashAddByte(sum, separatorByte)
		return true
	})

	attrs.Range(func(k string, v pcommon.Value) bool {
		sum = hashAdd(sum, k)
		sum = hashAddByte(sum, separatorByte)
		sum = hashAdd(sum, v.AsString())
		sum = hashAddByte(sum, separatorByte)
		return true
	})

	// add special __name__=name
	sum = hashAdd(sum, "__name__")
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, name)

	return sum
}
