package utils

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func GetTraceId(traceID pcommon.TraceID) string {
	if !traceID.IsEmpty() {
		return hex.EncodeToString(traceID[:])
	}
	return ""
}

func GetSpanId(spanID pcommon.SpanID) string {
	if !spanID.IsEmpty() {
		return hex.EncodeToString(spanID[:])
	}
	return ""
}
