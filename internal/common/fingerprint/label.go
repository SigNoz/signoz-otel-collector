package fingerprint

import (
	"slices"
	"sync"
)

const nameLabel = "__name__"

// The key slice and the output buffer are both scratch space whose lifetime
// ends when the JSON string is copied out, so they are pooled and reused across
// the millions of calls a single export batch makes.
var (
	labelKeysPool = sync.Pool{New: func() any { s := make([]string, 0, 64); return &s }}
	labelBufPool  = sync.Pool{New: func() any { b := make([]byte, 0, 512); return &b }}
)

// NewLabelsAsJSONString renders name and the given attribute maps as a single
// JSON object whose keys are sorted ascending. On a key present in more than
// one map the later map wins, and __name__ is always set to name regardless of
// what the maps carry. The result is byte-identical to marshaling the merged,
// sorted map, but it never materializes that merged map: the keys are gathered
// into one slice, sorted once, and the values are looked up directly from the
// source maps.
//
// It is significantly faster than json.Marshal and is compatible with
// ClickHouse JSON functions: https://clickhouse.yandex/docs/en/functions/json_functions.html
func NewLabelsAsJSONString(name string, ms ...map[string]string) string {
	// Gather every key (plus __name__) into one pooled slice, sort it, and emit
	// each unique key once, looking its value up directly from the source maps.
	kp := labelKeysPool.Get().(*[]string)
	keys := append((*kp)[:0], nameLabel)
	for _, m := range ms {
		for k := range m {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	bp := labelBufPool.Get().(*[]byte)
	b := append((*bp)[:0], '{')
	prev := ""
	for i, k := range keys {
		if i > 0 && k == prev {
			// same key contributed by more than one map; it was already emitted
			// with the winning value on its first (sorted) occurrence
			continue
		}
		prev = k

		if len(b) > 1 {
			b = append(b, ',')
		}
		b = append(b, '"')
		b = append(b, k...)
		b = append(b, '"', ':', '"')
		b = appendEscapedJSONValue(b, valueForKey(name, k, ms))
		b = append(b, '"')
	}
	b = append(b, '}')
	out := string(b)

	*kp = keys
	labelKeysPool.Put(kp)
	*bp = b
	labelBufPool.Put(bp)

	return out
}

// valueForKey returns the value to emit for k. __name__ always resolves to
// name; for every other key the last map that carries it wins, matching the
// "later map overwrites" semantics of merging the maps in order.
func valueForKey(name, k string, ms []map[string]string) string {
	if k == nameLabel {
		return name
	}
	for i := len(ms) - 1; i >= 0; i-- {
		if v, ok := ms[i][k]; ok {
			return v
		}
	}
	return ""
}

// appendEscapedJSONValue appends value to b, escaping the runes that must be
// escaped inside a JSON string. Label names are known not to contain such
// runes, so only values go through here.
func appendEscapedJSONValue(b []byte, value string) []byte {
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch c {
		case '\\', '"':
			b = append(b, '\\', c)
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, c)
		}
	}
	return b
}
