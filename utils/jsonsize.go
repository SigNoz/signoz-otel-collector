package utils

import (
	"encoding/base64"
	encodingjson "encoding/json"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const maxStackKeys = 32

// goccyJSON differs from stdJSON in writing \b and \f as \u0008 and \u000c, and keeping the leading zero in exponents such as 1e-07.
type jsonEncoder int

const (
	stdJSON jsonEncoder = iota
	goccyJSON
)

// AsStringSize returns len(v.AsString()) without building the string.
func AsStringSize(v pcommon.Value) int {
	switch v.Type() {
	case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
		n, ok := jsonSize(v, stdJSON)
		if !ok {
			return 0
		}
		return n
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodedLen(v.Bytes().Len())
	default:
		return len(v.AsString())
	}
}

// JSONSize returns len(encoding/json.Marshal(v.AsRaw())) without marshalling, and false where Marshal fails on NaN or Inf.
func JSONSize(v pcommon.Value) (int, bool) {
	return jsonSize(v, stdJSON)
}

// GoccyJSONSize is JSONSize for github.com/goccy/go-json's Marshal.
func GoccyJSONSize(v pcommon.Value) (int, bool) {
	return jsonSize(v, goccyJSON)
}

// JSONMapSize is JSONSize for a map, leaving out top-level keys for which skip returns true. skip may be nil.
func JSONMapSize(m pcommon.Map, skip func(key string) bool) (int, bool) {
	return jsonMapSize(m, skip, stdJSON)
}

// GoccyJSONMapSize is JSONMapSize for github.com/goccy/go-json's Marshal.
func GoccyJSONMapSize(m pcommon.Map, skip func(key string) bool) (int, bool) {
	return jsonMapSize(m, skip, goccyJSON)
}

func jsonSize(v pcommon.Value, enc jsonEncoder) (int, bool) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		return len("null"), true
	case pcommon.ValueTypeStr:
		return jsonStringSize(v.Str(), enc), true
	case pcommon.ValueTypeBool:
		if v.Bool() {
			return len("true"), true
		}
		return len("false"), true
	case pcommon.ValueTypeInt:
		var buf [20]byte
		return len(strconv.AppendInt(buf[:0], v.Int(), 10)), true
	case pcommon.ValueTypeDouble:
		return jsonFloatSize(v.Double(), enc)
	case pcommon.ValueTypeBytes:
		// AsRaw returns nil for empty bytes, which encodes as null
		if v.Bytes().Len() == 0 {
			return len("null"), true
		}
		return base64.StdEncoding.EncodedLen(v.Bytes().Len()) + 2, true
	case pcommon.ValueTypeMap:
		return jsonMapSize(v.Map(), nil, enc)
	case pcommon.ValueTypeSlice:
		return jsonSliceSize(v.Slice(), enc)
	}
	return 0, false
}

func jsonMapSize(m pcommon.Map, skip func(key string) bool, enc jsonEncoder) (int, bool) {
	if hasDuplicateKeys(m) {
		// AsRaw keeps the last value per key
		raw := m.AsRaw()
		if skip != nil {
			for k := range raw {
				if skip(k) {
					delete(raw, k)
				}
			}
		}
		var b []byte
		var err error
		if enc == goccyJSON {
			b, err = json.Marshal(raw)
		} else {
			b, err = encodingjson.Marshal(raw)
		}
		return len(b), err == nil
	}

	n, ok := len("{}"), true
	written := 0
	m.Range(func(k string, v pcommon.Value) bool {
		if skip != nil && skip(k) {
			return true
		}
		vn, vok := jsonSize(v, enc)
		if !vok {
			ok = false
			return false
		}
		if written > 0 {
			n++ // ,
		}
		n += jsonStringSize(k, enc) + 1 + vn // "key":value
		written++
		return true
	})
	return n, ok
}

func jsonSliceSize(s pcommon.Slice, enc jsonEncoder) (int, bool) {
	n := len("[]")
	for i := 0; i < s.Len(); i++ {
		vn, ok := jsonSize(s.At(i), enc)
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
func jsonFloatSize(f float64, enc jsonEncoder) (int, bool) {
	if !IsValidFloat(f) {
		return 0, false
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], f, format, -1, 64)
	n := len(b)
	// encoding/json writes e-07 as e-7, goccy keeps it
	if enc == stdJSON && format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		n--
	}
	return n, true
}

// jsonStringSize follows encoding/json's appendString with HTML escaping on.
func jsonStringSize(s string, enc jsonEncoder) int {
	n := len(`""`)
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			switch {
			case b >= 0x20 && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&':
				n++
			case b == '"' || b == '\\' || b == '\n' || b == '\r' || b == '\t':
				n += 2
			case (b == '\b' || b == '\f') && enc == stdJSON:
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
