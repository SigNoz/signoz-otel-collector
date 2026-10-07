package fieldvalues

import (
	"math"
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
// the message field and keys that look like ids, stops at the depth limit,
// and skips maps with too many keys, long arrays and nested arrays.
func bodyPairs(dst []pair, body pcommon.Value, limits BodyJSONLimits) []pair {
	if body.Type() != pcommon.ValueTypeMap {
		return dst
	}
	return walkBody(dst, "", body, 0, limits)
}

func walkBody(dst []pair, prefix string, val pcommon.Value, level int, limits BodyJSONLimits) []pair {
	if prefix == bodyMessageField || strings.HasPrefix(prefix, bodyMessageField+".") {
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
		var primitives []pcommon.Value
		var objects []pcommon.Value
		for i := 0; i < s.Len(); i++ {
			el := s.At(i)
			switch el.Type() {
			case pcommon.ValueTypeMap:
				objects = append(objects, el)
			case pcommon.ValueTypeSlice:
				return dst
			case pcommon.ValueTypeEmpty:
			default:
				primitives = append(primitives, el)
			}
		}
		for _, el := range objects {
			dst = walkBody(dst, prefix+bodyArraySuffix, el, level, limits)
		}
		for _, el := range primitives {
			dst = appendBodyValue(dst, prefix, el)
		}
		return dst
	default:
		return appendBodyValue(dst, prefix, val)
	}
}

func appendBodyValue(dst []pair, path string, val pcommon.Value) []pair {
	switch val.Type() {
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		return append(dst, stringPair(contextBody, path, val.AsString()))
	case pcommon.ValueTypeInt:
		return append(dst, numberPair(contextBody, path, float64(val.Int())))
	case pcommon.ValueTypeDouble:
		if f := val.Double(); !math.IsNaN(f) && !math.IsInf(f, 0) {
			return append(dst, numberPair(contextBody, path, f))
		}
	case pcommon.ValueTypeBool:
		return append(dst, boolPair(contextBody, path, val.Bool()))
	}
	return dst
}

// BodyValues lists what the walk of a JSON body finds, as "path=value", with
// "path=bool" for a bool value. The metadata exporter uses it to check the
// walk against its JSON writer.
func BodyValues(body pcommon.Value, limits BodyJSONLimits) []string {
	var out []string
	for _, p := range bodyPairs(nil, body, limits) {
		switch p.typ {
		case typeNumber:
			out = append(out, p.name+"="+strconv.FormatFloat(p.num, 'g', -1, 64))
		case typeBool:
			out = append(out, p.name+"=bool")
		default:
			out = append(out, p.name+"="+p.str)
		}
	}
	return out
}
