package fingerprint

import (
	"slices"
	"sync"
)

const nameLabel = "__name__"

// Pooled scratch space for the key slice and output buffer.
var (
	labelKeysPool = sync.Pool{New: func() any { s := make([]string, 0, 64); return &s }}
	labelBufPool  = sync.Pool{New: func() any { b := make([]byte, 0, 512); return &b }}
)

// NewLabelsAsJSONString renders name and the attribute maps as a single JSON
// object with keys sorted ascending. Later maps win on duplicate keys, and
// __name__ is always set to name.
//
// Compatible with ClickHouse JSON functions: https://clickhouse.yandex/docs/en/functions/json_functions.html
func NewLabelsAsJSONString(name string, ms ...map[string]string) string {
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
			// duplicate key, already emitted with the winning value
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

// valueForKey returns the value to emit for k: name for __name__, else the last map that carries it.
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

// appendEscapedJSONValue appends value to b, escaping runes that require escaping inside a JSON string.
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
