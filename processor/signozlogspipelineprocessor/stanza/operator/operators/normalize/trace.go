package json

import (
	"encoding/hex"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

const (
	traceIDSize    = 16
	spanIDSize     = 8
	traceFlagsSize = 1
)

func (f fieldConfig) setTraceContext(ent *entry.Entry, results scanResults) {
	if id, ok := f.take(results, targetTraceID); ok {
		ent.TraceID = id.([]byte)
	}
	if id, ok := f.take(results, targetSpanID); ok {
		ent.SpanID = id.([]byte)
	}
	if flags, ok := f.take(results, targetTraceFlags); ok {
		ent.TraceFlags = flags.([]byte)
	}
}

func parseTraceID(value any) (any, bool) {
	return parseID(value, traceIDSize)
}

func parseSpanID(value any) (any, bool) {
	return parseID(value, spanIDSize)
}

func parseID(value any, size int) (any, bool) {
	var id []byte

	switch v := value.(type) {
	case string:
		if len(v) != hex.EncodedLen(size) {
			return nil, false
		}
		decoded, err := hex.DecodeString(v)
		if err != nil {
			return nil, false
		}
		id = decoded
	case []byte:
		if len(v) != size {
			return nil, false
		}
		id = v
	default:
		return nil, false
	}

	for _, b := range id {
		if b != 0 {
			return id, true
		}
	}
	return nil, false
}

func parseTraceFlags(value any) (any, bool) {
	switch v := value.(type) {
	case string:
		if len(v) != hex.EncodedLen(traceFlagsSize) {
			return nil, false
		}
		decoded, err := hex.DecodeString(v)
		if err != nil {
			return nil, false
		}
		return decoded, true
	case []byte:
		if len(v) != traceFlagsSize {
			return nil, false
		}
		return v, true
	case int64:
		return traceFlagsFromNumber(v)
	case int:
		return traceFlagsFromNumber(int64(v))
	case float64:
		if v != float64(int64(v)) {
			return nil, false
		}
		return traceFlagsFromNumber(int64(v))
	default:
		return nil, false
	}
}

func traceFlagsFromNumber(number int64) (any, bool) {
	if number < 0 || number > 0xff {
		return nil, false
	}
	return []byte{byte(number)}, true
}
