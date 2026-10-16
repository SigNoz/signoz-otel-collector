package signozlogsnormalizerprocessor

import (
	"encoding/base64"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const hexDigits = "0123456789abcdef"

type mapEntry struct {
	key   string
	value pcommon.Value
}

func appendAsString(dst []byte, v pcommon.Value) ([]byte, bool) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		return append(dst, "null"...), true
	case pcommon.ValueTypeStr:
		return appendJSONString(dst, v.Str()), true
	case pcommon.ValueTypeBool:
		return strconv.AppendBool(dst, v.Bool()), true
	case pcommon.ValueTypeInt:
		return strconv.AppendInt(dst, v.Int(), 10), true
	case pcommon.ValueTypeDouble:
		return appendJSONFloat(dst, v.Double())
	case pcommon.ValueTypeBytes:
		if v.Bytes().Len() == 0 {
			return append(dst, "null"...), true
		}
		dst = append(dst, '"')
		dst = base64.StdEncoding.AppendEncode(dst, v.Bytes().AsRaw())
		return append(dst, '"'), true
	case pcommon.ValueTypeSlice:
		s := v.Slice()
		if s.Len() == 0 {
			return append(dst, "[]"...), true
		}
		dst = append(dst, '[')
		for i := 0; i < s.Len(); i++ {
			if i > 0 {
				dst = append(dst, ',')
			}
			var ok bool
			if dst, ok = appendAsString(dst, s.At(i)); !ok {
				return dst, false
			}
		}
		return append(dst, ']'), true
	case pcommon.ValueTypeMap:
		return appendMapAsString(dst, v.Map())
	}
	return dst, false
}

func appendMapAsString(dst []byte, m pcommon.Map) ([]byte, bool) {
	entries := make([]mapEntry, 0, m.Len())
	m.Range(func(k string, v pcommon.Value) bool {
		entries = append(entries, mapEntry{key: k, value: v})
		return true
	})
	slices.SortStableFunc(entries, func(a, b mapEntry) int { return strings.Compare(a.key, b.key) })

	dst = append(dst, '{')
	first := true
	for i, e := range entries {
		if i+1 < len(entries) && entries[i+1].key == e.key {
			continue
		}
		if !first {
			dst = append(dst, ',')
		}
		first = false
		dst = appendJSONString(dst, e.key)
		dst = append(dst, ':')
		var ok bool
		if dst, ok = appendAsString(dst, e.value); !ok {
			return dst, false
		}
	}
	return append(dst, '}'), true
}

func appendJSONFloat(dst []byte, f float64) ([]byte, bool) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return dst, false
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	dst = strconv.AppendFloat(dst, f, format, -1, 64)
	if format == 'e' {
		if n := len(dst); n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst, true
}

func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= 0x20 && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch b {
			case '\\', '"':
				dst = append(dst, '\\', b)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
			continue
		}
		if c == ' ' || c == ' ' {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}
