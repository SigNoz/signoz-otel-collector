package utils

import (
	"encoding/hex"
	encodingjson "encoding/json"
	"math"
	"runtime"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TraceIDToHexOrEmptyString(traceID pcommon.TraceID) string {
	if !traceID.IsEmpty() {
		return hex.EncodeToString(traceID[:])
	}
	return ""
}

func SpanIDToHexOrEmptyString(spanID pcommon.SpanID) string {
	if !spanID.IsEmpty() {
		return hex.EncodeToString(spanID[:])
	}
	return ""
}

func IsValidFloat(value float64) bool {
	// Check for NaN, +/-Inf
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

const (
	// ClickHouse rejects JSON text nested 1024 levels deep, and 149 directly nested arrays.
	maxJSONDepth      = 1000
	maxJSONArrayDepth = 128
	noJSONDepthLimit  = math.MinInt
)

// SanitizeJSON rewrites, in place, the values ClickHouse rejects in JSON text:
// NaN/Inf become null (encoding/json errors out on them), float64 values outside the Int64 range
// are written in exponent form, since Go prints them as integer literals ClickHouse cannot store,
// and maps or slices nested past maxJSONDepth, or slices past maxJSONArrayDepth, become their JSON text.
func SanitizeJSON(v any) any {
	return sanitizeJSON(v, 0, 0)
}

func sanitizeJSON(v any, depth, arrayDepth int) any {
	switch val := v.(type) {
	case float64:
		if !IsValidFloat(val) {
			return nil
		}
		if val >= 1<<63 || val <= -(1<<63) {
			return json.Number(strconv.FormatFloat(val, 'e', -1, 64))
		}
		return val
	case map[string]any:
		if depth >= maxJSONDepth {
			return jsonText(val)
		}
		for k, vv := range val {
			val[k] = sanitizeJSON(vv, depth+1, 0)
		}
		return val
	case []any:
		if depth >= maxJSONDepth || arrayDepth >= maxJSONArrayDepth {
			return jsonText(val)
		}
		for i, vv := range val {
			val[i] = sanitizeJSON(vv, depth+1, arrayDepth+1)
		}
		return val
	default:
		return v
	}
}

func jsonText(v any) string {
	b, err := json.Marshal(sanitizeJSON(v, noJSONDepthLimit, noJSONDepthLimit))
	if err != nil {
		return ""
	}
	return string(b)
}

type TagType string

const (
	TagTypeAttribute TagType = "tag"
	TagTypeScope     TagType = "scope"
	TagTypeResource  TagType = "resource"

	TagTypeSpanField TagType = "spanfield"
	TagTypeLogField  TagType = "logfield"
	TagTypeBodyField TagType = "body"
)

func (t TagType) String() string {
	return string(t)
}

type FieldDataType string

const (
	FieldDataTypeString       FieldDataType = "string"
	FieldDataTypeFloat64      FieldDataType = "float64"
	FieldDataTypeInt64        FieldDataType = "int64"
	FieldDataTypeBool         FieldDataType = "bool"
	FieldDataTypeArrayString  FieldDataType = "[]string"
	FieldDataTypeArrayInt64   FieldDataType = "[]int64"
	FieldDataTypeArrayFloat64 FieldDataType = "[]float64"
	FieldDataTypeArrayBool    FieldDataType = "[]bool"
	FieldDataTypeArrayJSON    FieldDataType = "[]json"
	FieldDataTypeArrayDynamic FieldDataType = "[]dynamic"
)

func (t FieldDataType) String() string {
	return string(t)
}

func MakeKeyForRFCache(bucketTs int64, fingerprint string) string {
	var v strings.Builder
	v.WriteString(strconv.Itoa(int(bucketTs)))
	v.WriteString(":")
	v.WriteString(fingerprint)
	return v.String()
}

func MakeKeyForAttributeKeys(tagKey string, tagType TagType, dataType FieldDataType) string {
	var key strings.Builder
	key.WriteString(tagKey)
	key.WriteString(":")
	key.WriteString(string(tagType))
	key.WriteString(":")
	key.WriteString(string(dataType))
	return key.String()
}

func MakeKeyForSkipKeys(context TagType, tagKey string) string {
	var key strings.Builder
	key.WriteString(string(context))
	key.WriteString(":")
	key.WriteString(tagKey)
	return key.String()
}

// Unquote attempts to unquote the string if not then returns the original
func Unquote(value string) string {
	unquoted, err := strconv.Unquote(value)
	if err == nil {
		return unquoted
	}

	return value
}

// IsJSON works with encoding/json instead of goccy because of better benchmarking specifically for Valid function with default package
// read here: https://github.com/SigNoz/signoz-otel-collector/pull/641#issuecomment-3068629067
func IsJSON(v any) bool {
	switch val := v.(type) {
	case string:
		return encodingjson.Valid([]byte(val))
	case []byte:
		return encodingjson.Valid(val)
	}

	return false
}

func Concurrency() int {
	return int(math.Max(1, float64(runtime.GOMAXPROCS(0))))
}

func ToPointer[T any](v T) *T {
	return &v
}

func ToLookUpMap[T comparable](s []T) map[T]struct{} {
	lookUp := map[T]struct{}{}
	for _, e := range s {
		lookUp[e] = struct{}{}
	}
	return lookUp
}
