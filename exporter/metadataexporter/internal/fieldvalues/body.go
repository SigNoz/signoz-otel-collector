package fieldvalues

import (
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
)

// BodyJSONLimits are the limits of the walk over a JSON log body. They are
// the limits of the JSON writer of the metadata exporter, so both see the
// same paths.
type BodyJSONLimits struct {
	MaxDepthTraverse        int
	MaxArrayElementsAllowed int
	MaxKeysAtLevel          int
}

const (
	bodyArraySuffix  = "[]"
	bodyMessageField = "message"
)

// bodyPairs gives the pairs of a JSON log body: one pair for each primitive
// value, named by its dotted path. The walk follows the JSON writer: it skips
// keys that look like ids, stops at the depth limit, and skips maps with too
// many keys, long arrays and nested arrays.
//
// The keys also follow the JSON writer. An array has one key, with the type
// of the array. The message field and an array of objects have a key and no
// value: their pair has the empty string, which is never a value.
func bodyPairs(dst []pair, body pcommon.Value, limits BodyJSONLimits) []pair {
	if body.Type() != pcommon.ValueTypeMap {
		return dst
	}
	return walkBody(dst, "", body, 0, limits)
}

func walkBody(dst []pair, prefix string, val pcommon.Value, level int, limits BodyJSONLimits) []pair {
	if prefix == bodyMessageField {
		return append(dst, stringPair(contextBody, prefix, ""))
	}
	if strings.HasPrefix(prefix, bodyMessageField+".") {
		return dst
	}
	if level > limits.MaxDepthTraverse && (val.Type() == pcommon.ValueTypeMap || val.Type() == pcommon.ValueTypeSlice) {
		return dst
	}
	switch val.Type() {
	case pcommon.ValueTypeMap:
		m := val.Map()
		if m.Len() > limits.MaxKeysAtLevel {
			return dst
		}
		m.Range(func(key string, child pcommon.Value) bool {
			if keycheck.IsCardinal(key) {
				return true
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			dst = walkBody(dst, path, child, level+2, limits)
			return true
		})
		return dst
	case pcommon.ValueTypeSlice:
		s := val.Slice()
		if s.Len() == 0 || s.Len() > limits.MaxArrayElementsAllowed {
			return dst
		}
		// A nested array ends the walk of the array with no key. The objects
		// before it are walked, as in the JSON writer.
		var objects, strs, floats, ints, bools bool
		for i := 0; i < s.Len(); i++ {
			el := s.At(i)
			switch el.Type() {
			case pcommon.ValueTypeMap:
				dst = walkBody(dst, prefix+bodyArraySuffix, el, level, limits)
				objects = true
			case pcommon.ValueTypeSlice:
				return dst
			case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
				strs = true
			case pcommon.ValueTypeDouble:
				floats = true
			case pcommon.ValueTypeInt:
				ints = true
			case pcommon.ValueTypeBool:
				bools = true
			}
		}
		primitives := strs || floats || ints || bools
		var array arrayType
		switch {
		case objects && primitives:
			array = arrayDynamic
		case objects:
			array = arrayJSON
		case strs && (floats || ints || bools):
			array = arrayDynamic
		case strs:
			array = arrayString
		case floats:
			array = arrayFloat64
		case ints:
			array = arrayInt64
		case bools:
			array = arrayBool
		default:
			return dst
		}
		if !primitives {
			p := stringPair(contextBody, prefix, "")
			p.array = array
			return append(dst, p)
		}
		for i := 0; i < s.Len(); i++ {
			dst = appendBodyValue(dst, prefix, s.At(i), array)
		}
		return dst
	default:
		return appendBodyValue(dst, prefix, val, arrayNone)
	}
}

func appendBodyValue(dst []pair, path string, val pcommon.Value, array arrayType) []pair {
	var p pair
	switch val.Type() {
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		p = stringPair(contextBody, path, val.AsString())
	case pcommon.ValueTypeInt:
		p = intPair(contextBody, path, val.Int())
	case pcommon.ValueTypeDouble:
		p = numberPair(contextBody, path, val.Double())
	case pcommon.ValueTypeBool:
		p = boolPair(contextBody, path, val.Bool())
	default:
		return dst
	}
	p.array = array
	return append(dst, p)
}

// BodyValues lists the values that the walk of a JSON body finds, as
// "path=value", with "path=bool" for a bool value. The metadata exporter uses
// it to check the walk against its JSON writer.
func BodyValues(body pcommon.Value, limits BodyJSONLimits) []string {
	var out []string
	for _, p := range bodyPairs(nil, body, limits) {
		switch {
		case p.typ == typeNumber:
			if hasValue(p.num) {
				out = append(out, p.name+"="+strconv.FormatFloat(p.num, 'g', -1, 64))
			}
		case p.typ == typeBool:
			out = append(out, p.name+"=bool")
		case p.str != "":
			out = append(out, p.name+"="+p.str)
		}
	}
	return out
}

// BodyKeys lists the keys that the walk of a JSON body finds, as "path type".
// A key that the walk finds more than once is listed more than once.
func BodyKeys(body pcommon.Value, limits BodyJSONLimits) []string {
	var out []string
	for _, p := range bodyPairs(nil, body, limits) {
		out = append(out, p.name+" "+p.keyDataType())
	}
	return out
}
