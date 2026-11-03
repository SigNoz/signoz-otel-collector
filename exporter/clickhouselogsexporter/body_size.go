package clickhouselogsexporter

import (
	"encoding/base64"
	encodingjson "encoding/json"
	"math"
	"strconv"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// maxStackKeys is the map size up to which duplicate keys are checked without allocating.
const maxStackKeys = 32

// bodySize returns len(body.AsString()) without building the string.
// AsString marshals maps and slices with encoding/json, so the sizes below follow its encoding:
// key order does not change the length, and a NaN or Inf anywhere fails Marshal, which makes AsString return "".
func bodySize(body pcommon.Value) int {
	switch body.Type() {
	case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
		n, ok := jsonValueSize(body)
		if !ok {
			return 0
		}
		return n
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodedLen(body.Bytes().Len())
	default:
		return len(body.AsString())
	}
}

// jsonValueSize returns the length of encoding/json's output for v.AsRaw(), and false where Marshal would fail.
func jsonValueSize(v pcommon.Value) (int, bool) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		return len("null"), true
	case pcommon.ValueTypeStr:
		return jsonStringSize(v.Str()), true
	case pcommon.ValueTypeBool:
		if v.Bool() {
			return len("true"), true
		}
		return len("false"), true
	case pcommon.ValueTypeInt:
		var buf [20]byte
		return len(strconv.AppendInt(buf[:0], v.Int(), 10)), true
	case pcommon.ValueTypeDouble:
		return jsonFloatSize(v.Double())
	case pcommon.ValueTypeBytes:
		// AsRaw returns nil for empty bytes, which encodes as null
		if v.Bytes().Len() == 0 {
			return len("null"), true
		}
		return base64.StdEncoding.EncodedLen(v.Bytes().Len()) + 2, true
	case pcommon.ValueTypeMap:
		return jsonMapSize(v.Map())
	case pcommon.ValueTypeSlice:
		return jsonSliceSize(v.Slice())
	}
	return 0, false
}

func jsonMapSize(m pcommon.Map) (int, bool) {
	if hasDuplicateKeys(m) {
		// AsRaw keeps the last value per key, so let encoding/json measure what is left
		b, err := encodingjson.Marshal(m.AsRaw())
		return len(b), err == nil
	}

	n, ok := len("{}"), true
	i := 0
	m.Range(func(k string, v pcommon.Value) bool {
		vn, vok := jsonValueSize(v)
		if !vok {
			ok = false
			return false
		}
		if i > 0 {
			n++ // ,
		}
		n += jsonStringSize(k) + 1 + vn // "key":value
		i++
		return true
	})
	return n, ok
}

func jsonSliceSize(s pcommon.Slice) (int, bool) {
	n := len("[]")
	for i := 0; i < s.Len(); i++ {
		vn, ok := jsonValueSize(s.At(i))
		if !ok {
			return 0, false
		}
		if i > 0 {
			n++ // ,
		}
		n += vn
	}
	return n, true
}

func hasDuplicateKeys(m pcommon.Map) bool {
	if m.Len() < 2 {
		return false
	}
	if m.Len() > maxStackKeys {
		seen := make(map[string]struct{}, m.Len())
		dup := false
		m.Range(func(k string, _ pcommon.Value) bool {
			if _, ok := seen[k]; ok {
				dup = true
				return false
			}
			seen[k] = struct{}{}
			return true
		})
		return dup
	}

	var keys [maxStackKeys]string
	n := 0
	dup := false
	m.Range(func(k string, _ pcommon.Value) bool {
		for _, seen := range keys[:n] {
			if seen == k {
				dup = true
				return false
			}
		}
		keys[n] = k
		n++
		return true
	})
	return dup
}

// jsonFloatSize follows encoding/json's floatEncoder for float64.
func jsonFloatSize(f float64) (int, bool) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], f, format, -1, 64)
	n := len(b)
	// encoding/json writes e-07 as e-7
	if format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		n--
	}
	return n, true
}

// jsonStringSize follows encoding/json's appendString with HTML escaping on.
func jsonStringSize(s string) int {
	n := len(`""`)
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			switch {
			case b >= 0x20 && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&':
				n++
			case b == '"' || b == '\\' || b == '\b' || b == '\f' || b == '\n' || b == '\r' || b == '\t':
				n += 2
			default:
				n += len(`\u00XX`)
			}
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case c == utf8.RuneError && size == 1:
			n += len(`\ufffd`)
		case c == '\u2028' || c == '\u2029':
			n += len(`\u2028`)
		default:
			n += size
		}
		i += size
	}
	return n
}
