package signozlogsinferrerprocessor

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func setTraceContext(lr plog.LogRecord, results scanResults) {
	if results.found[targetTraceID] {
		lr.SetTraceID(results.values[targetTraceID].traceID)
	}
	if results.found[targetSpanID] {
		lr.SetSpanID(results.values[targetSpanID].spanID)
	}
}

func parseTraceID(value pcommon.Value) (inferred, bool) {
	var id pcommon.TraceID
	if !parseID(value, id[:]) {
		return inferred{}, false
	}
	return inferred{traceID: id}, true
}

func parseSpanID(value pcommon.Value) (inferred, bool) {
	var id pcommon.SpanID
	if !parseID(value, id[:]) {
		return inferred{}, false
	}
	return inferred{spanID: id}, true
}

func parseID(value pcommon.Value, dst []byte) bool {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		str := value.Str()
		if len(str) != hex.EncodedLen(len(dst)) {
			return false
		}
		if _, err := hex.Decode(dst, []byte(str)); err != nil {
			return false
		}
	case pcommon.ValueTypeBytes:
		raw := value.Bytes().AsRaw()
		if len(raw) != len(dst) {
			return false
		}
		copy(dst, raw)
	default:
		return false
	}

	for _, b := range dst {
		if b != 0 {
			return true
		}
	}
	return false
}
