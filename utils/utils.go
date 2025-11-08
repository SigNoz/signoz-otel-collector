package utils

import (
	"encoding/hex"
	encodingjson "encoding/json"
	"math"
	"runtime"
	"strconv"
	"strings"

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

type TagType string

const (
	TagTypeAttribute TagType = "tag"
	TagTypeScope     TagType = "scope"
	TagTypeResource  TagType = "resource"

	TagTypeSpanField TagType = "spanfield"
	TagTypeLogField  TagType = "logfield"
)

func (t TagType) String() string {
	return string(t)
}

type TagDataType string

const (
	TagDataTypeString TagDataType = "string"
	TagDataTypeBool   TagDataType = "bool"
	TagDataTypeNumber TagDataType = "float64"
)

func (t TagDataType) String() string {
	return string(t)
}

func MakeKeyForRFCache(bucketTs int64, fingerprint string) string {
	var v strings.Builder
	v.WriteString(strconv.Itoa(int(bucketTs)))
	v.WriteString(":")
	v.WriteString(fingerprint)
	return v.String()
}

func MakeKeyForAttributeKeys(tagKey string, tagType TagType, tagDataType TagDataType) string {
	var key strings.Builder
	key.WriteString(tagKey)
	key.WriteString(":")
	key.WriteString(string(tagType))
	key.WriteString(":")
	key.WriteString(string(tagDataType))
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
