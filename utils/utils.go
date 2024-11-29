package utils

import (
	"encoding/hex"
	"math"

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
