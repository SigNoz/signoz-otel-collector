package fingerprint

import (
	"fmt"
	"sort"
)

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

// FingerprintHash calculates a fingerprint of SORTED BY NAME labels.
// It is adopted from labelSetToFingerprint, but avoids type conversions and memory allocations.
func FingerprintHash(attribs map[string]any) uint64 {
	if len(attribs) == 0 {
		return offset64
	}

	keys := []string{}
	for k := range attribs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sum := offset64
	for _, k := range keys {
		sum = hashAdd(sum, k)
		sum = hashAddByte(sum, separatorByte)
		sum = hashAdd(sum, fmt.Sprintf("%v", attribs[k]))
		sum = hashAddByte(sum, separatorByte)
	}
	return sum
}
