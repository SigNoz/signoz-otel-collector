// brought in as is from "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
// with minor changes to use signozstanzaentry.Field instead of entry.Field

package signozstanzahelper

import (
	"encoding/hex"
	"fmt"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// NewTraceParser creates a new trace parser with default values
func NewTraceParser() TraceParser {
	traceID := signozstanzaentry.Field{FieldInterface: entry.NewBodyField("trace_id")}
	spanID := signozstanzaentry.Field{FieldInterface: entry.NewBodyField("span_id")}
	traceFlags := signozstanzaentry.Field{FieldInterface: entry.NewBodyField("trace_flags")}
	return TraceParser{
		TraceID: &TraceIDConfig{
			ParseFrom: &traceID,
		},
		SpanID: &SpanIDConfig{
			ParseFrom: &spanID,
		},
		TraceFlags: &TraceFlagsConfig{
			ParseFrom: &traceFlags,
		},
	}
}

// TraceParser is a helper that parses trace spans (and flags) onto an entry.
type TraceParser struct {
	TraceID    *TraceIDConfig    `mapstructure:"trace_id,omitempty"`
	SpanID     *SpanIDConfig     `mapstructure:"span_id,omitempty"`
	TraceFlags *TraceFlagsConfig `mapstructure:"trace_flags,omitempty"`
}

type TraceIDConfig struct {
	ParseFrom *signozstanzaentry.Field `mapstructure:"parse_from,omitempty"`
}

type SpanIDConfig struct {
	ParseFrom *signozstanzaentry.Field `mapstructure:"parse_from,omitempty"`
}

type TraceFlagsConfig struct {
	ParseFrom *signozstanzaentry.Field `mapstructure:"parse_from,omitempty"`
}

// Validate validates a TraceParser, and reconfigures it if necessary
func (t *TraceParser) Validate() error {
	if t.TraceID == nil {
		t.TraceID = &TraceIDConfig{}
	}
	if t.TraceID.ParseFrom == nil {
		field := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("trace_id")}
		t.TraceID.ParseFrom = &field
	}
	if t.SpanID == nil {
		t.SpanID = &SpanIDConfig{}
	}
	if t.SpanID.ParseFrom == nil {
		field := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("span_id")}
		t.SpanID.ParseFrom = &field
	}
	if t.TraceFlags == nil {
		t.TraceFlags = &TraceFlagsConfig{}
	}
	if t.TraceFlags.ParseFrom == nil {
		field := signozstanzaentry.Field{FieldInterface: signozstanzaentry.NewBodyField("trace_flags")}
		t.TraceFlags.ParseFrom = &field
	}
	return nil
}

// Best effort hex parsing for trace, spans and flags
func parseHexField(entry *entry.Entry, field *signozstanzaentry.Field) ([]byte, error) {
	value, ok := entry.Get(field)
	if !ok {
		return nil, nil
	}

	data, err := hex.DecodeString(fmt.Sprintf("%v", value))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Parse will parse a trace (trace_id, span_id and flags) from a field and attach it to the entry
func (t *TraceParser) Parse(entry *entry.Entry) error {
	var errTraceID, errSpanID, errTraceFlags error
	entry.TraceID, errTraceID = parseHexField(entry, t.TraceID.ParseFrom)
	entry.SpanID, errSpanID = parseHexField(entry, t.SpanID.ParseFrom)
	entry.TraceFlags, errTraceFlags = parseHexField(entry, t.TraceFlags.ParseFrom)
	if errTraceID != nil || errTraceFlags != nil || errSpanID != nil {
		err := errors.NewError("Error decoding traces for logs", "")
		if errTraceID != nil {
			_ = err.WithDetails("trace_id", errTraceID.Error())
		}
		if errSpanID != nil {
			_ = err.WithDetails("span_id", errSpanID.Error())
		}
		if errTraceFlags != nil {
			_ = err.WithDetails("trace_flags", errTraceFlags.Error())
		}
		return err
	}
	return nil
}
