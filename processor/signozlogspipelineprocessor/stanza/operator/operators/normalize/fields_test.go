package json

import (
	"context"
	"testing"
	"time"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func newInferenceConfig() *Config {
	cfg := NewConfig()
	cfg.SeverityNumberFields = []string{"severity_number", "severitynumber"}
	cfg.SeverityTextFields = []string{
		"severity_text", "severitytext", "severity",
		"level", "log.level", "log_level", "loglevel", "levelname", "lvl",
	}
	cfg.TraceIDFields = []string{"trace_id", "traceid", "trace.id"}
	cfg.SpanIDFields = []string{"span_id", "spanid", "span.id"}
	cfg.TraceFlagsFields = []string{"trace_flags", "traceflags", "trace.flags"}
	cfg.ScopeNameFields = []string{"scope.name", "scope_name", "scopename"}
	cfg.ScopeVersionFields = []string{"scope.version", "scope_version", "scopeversion"}
	return cfg
}

var defaultFields = newFieldConfig(*newInferenceConfig())

func newProcessorWithConfig(t testing.TB, cfg *Config) *Processor {
	t.Helper()
	cfg.OutputIDs = []string{"fake"}
	op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	processor := op.(*Processor)
	require.NoError(t, processor.SetOutputs([]operator.Operator{testutil.NewFakeOutput(t)}))
	return processor
}

func enableEveryMoveField(cfg *Config) {
	cfg.MoveSeverityNumberField = true
	cfg.MoveSeverityTextField = true
	cfg.MoveTraceIDField = true
	cfg.MoveSpanIDField = true
	cfg.MoveTraceFlagsField = true
	cfg.MoveScopeNameField = true
	cfg.MoveScopeVersionField = true
}

func TestConfiguredFieldNames(t *testing.T) {
	cfg := NewConfig()
	enableEveryMoveField(cfg)
	cfg.MessageFields = []string{"event", "msg"}
	cfg.SeverityNumberFields = []string{"SevNum"}
	cfg.SeverityTextFields = []string{"Sev"}
	cfg.TraceIDFields = []string{"correlation.trace"}
	cfg.SpanIDFields = []string{"correlation.span"}
	cfg.TraceFlagsFields = []string{"correlation.flags"}
	cfg.ScopeNameFields = []string{"component"}
	cfg.ScopeVersionFields = []string{"component_version"}
	processor := newProcessorWithConfig(t, cfg)

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{
		"event":             "boom",
		"sev":               "warning",
		"correlation.trace": testTraceID,
		"correlation.span":  testSpanID,
		"correlation.flags": "01",
		"component":         "checkout",
		"component_version": "1.4.0",
		"level":             "error",
		"trace_id":          "11111111111111111111111111111111",
		"trace_flags":       "ff",
		"logger":            "cart",
	}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{
		"message":     "boom",
		"level":       "error",
		"trace_id":    "11111111111111111111111111111111",
		"trace_flags": "ff",
		"logger":      "cart",
	}, e.Body)
	require.Equal(t, entry.Warn, e.Severity)
	require.Equal(t, "WARN", e.SeverityText)
	require.Equal(t, decodeHexOrPanic(testTraceID), e.TraceID)
	require.Equal(t, decodeHexOrPanic(testSpanID), e.SpanID)
	require.Equal(t, []byte{1}, e.TraceFlags)
	require.Equal(t, "checkout", e.ScopeName)
	require.Equal(t, "1.4.0", e.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute])
}

func TestConfiguredSeverityNumberField(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.SeverityNumberFields = []string{"sev_num"}
	cfg.SeverityTextFields = []string{"level"}
	processor := newProcessorWithConfig(t, cfg)

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{"sev_num": int64(17), "level": "info"}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, entry.Error, e.Severity)
	require.Equal(t, "INFO", e.SeverityText)
}

func TestMessageFieldNamesIncludeMessageItself(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.MessageFields = []string{"event"}
	processor := newProcessorWithConfig(t, cfg)

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{"Message": "not a candidate", "event": "boom"}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{"message": "boom", "Message": "not a candidate"}, e.Body)
}

func TestEmptyFieldListStopsInference(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.MessageFields = []string{}
	cfg.SeverityNumberFields = []string{}
	cfg.SeverityTextFields = []string{}
	cfg.TraceIDFields = []string{}
	processor := newProcessorWithConfig(t, cfg)

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{"msg": "boom", "level": "error", "trace_id": testTraceID}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{"msg": "boom", "level": "error", "trace_id": testTraceID}, e.Body)
	require.Equal(t, entry.Default, e.Severity)
	require.Empty(t, e.SeverityText)
	require.Empty(t, e.TraceID)
}

func TestMoveRemovesTheFieldItUsed(t *testing.T) {
	cases := []struct {
		name               string
		prepare            func(*entry.Entry)
		body               any
		attributes         map[string]any
		resource           map[string]any
		expectedBody       any
		expectedAttributes map[string]any
		expectedResource   map[string]any
	}{
		{
			name:         "from_body",
			body:         map[string]any{"message": "boom", "level": "error", "trace_id": testTraceID, "scope_name": "cart"},
			expectedBody: map[string]any{"message": "boom"},
		},
		{
			name:               "from_attributes",
			body:               map[string]any{"message": "boom"},
			attributes:         map[string]any{"level": "error", "trace_id": testTraceID, "span_id": testSpanID, "trace_flags": "01", "kept": "value"},
			expectedBody:       map[string]any{"message": "boom"},
			expectedAttributes: map[string]any{"kept": "value"},
		},
		{
			name:             "from_resource",
			body:             map[string]any{"message": "boom"},
			resource:         map[string]any{"scope_name": "checkout", "service.name": "checkout"},
			expectedBody:     map[string]any{"message": "boom"},
			expectedResource: map[string]any{"service.name": "checkout"},
		},
		{
			name:         "both_severity_fields_when_both_are_read",
			body:         map[string]any{"message": "boom", "severity_number": int64(17), "level": "info"},
			expectedBody: map[string]any{"message": "boom"},
		},
		{
			name:         "only_the_field_that_was_used",
			body:         map[string]any{"message": "boom", "level": "info", "lvl": "error"},
			expectedBody: map[string]any{"message": "boom", "lvl": "error"},
		},
		{
			name: "nothing_is_removed_when_the_record_already_has_the_field",
			prepare: func(e *entry.Entry) {
				e.Severity = entry.Info
				e.SeverityText = "INFO"
				e.TraceID = decodeHexOrPanic("11111111111111111111111111111111")
			},
			body:               map[string]any{"message": "boom", "level": "error"},
			attributes:         map[string]any{"trace_id": testTraceID},
			expectedBody:       map[string]any{"message": "boom", "level": "error"},
			expectedAttributes: map[string]any{"trace_id": testTraceID},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newInferenceConfig()
			enableEveryMoveField(cfg)
			processor := newProcessorWithConfig(t, cfg)

			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource
			if tc.prepare != nil {
				tc.prepare(e)
			}

			require.NoError(t, processor.Process(context.Background(), e))

			require.Equal(t, tc.expectedBody, e.Body)
			require.Equal(t, tc.expectedAttributes, e.Attributes)
			require.Equal(t, tc.expectedResource, e.Resource)
		})
	}
}

func TestMoveRemovesTheFieldFromScopeAttributes(t *testing.T) {
	cfg := newInferenceConfig()
	enableEveryMoveField(cfg)
	processor := newProcessorWithConfig(t, cfg)

	scope := map[string]any{
		"level":         "error",
		"trace_id":      testTraceID,
		"scope_version": "1.4.0",
		"kept":          "value",
	}
	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{"message": "boom"}
	withScopeAttributes(e, scope)
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{"kept": "value"}, scope)
}

func TestOnlyTheMessageFieldIsMovedByDefault(t *testing.T) {
	processor := newProcessorWithConfig(t, newInferenceConfig())

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{
		"msg":           "boom",
		"level":         "error",
		"trace_id":      testTraceID,
		"span_id":       testSpanID,
		"trace_flags":   "01",
		"scope_name":    "cart",
		"scope_version": "1.4.0",
	}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{
		"message":       "boom",
		"level":         "error",
		"trace_id":      testTraceID,
		"span_id":       testSpanID,
		"trace_flags":   "01",
		"scope_name":    "cart",
		"scope_version": "1.4.0",
	}, e.Body)

	require.Equal(t, entry.Error, e.Severity)
	require.Equal(t, "ERROR", e.SeverityText)
	require.Equal(t, decodeHexOrPanic(testTraceID), e.TraceID)
	require.Equal(t, decodeHexOrPanic(testSpanID), e.SpanID)
	require.Equal(t, []byte{1}, e.TraceFlags)
	require.Equal(t, "cart", e.ScopeName)
	require.Equal(t, "1.4.0", e.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute])
}

func TestMoveAllFieldsOverridesTheIndividualSettings(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.MoveAllFields = true
	cfg.MoveSeverityTextField = false
	processor := newProcessorWithConfig(t, cfg)

	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{
		"msg":           "boom",
		"level":         "error",
		"trace_id":      testTraceID,
		"span_id":       testSpanID,
		"trace_flags":   "01",
		"scope_name":    "cart",
		"scope_version": "1.4.0",
		"kept":          "value",
	}
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, map[string]any{"message": "boom", "kept": "value"}, e.Body)
	require.Equal(t, entry.Error, e.Severity)
	require.Equal(t, "ERROR", e.SeverityText)
	require.Equal(t, decodeHexOrPanic(testTraceID), e.TraceID)
	require.Equal(t, decodeHexOrPanic(testSpanID), e.SpanID)
	require.Equal(t, []byte{1}, e.TraceFlags)
	require.Equal(t, "cart", e.ScopeName)
	require.Equal(t, "1.4.0", e.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute])
}

func TestMoveIsPerField(t *testing.T) {
	cases := []struct {
		name         string
		move         func(*Config)
		expectedBody map[string]any
	}{
		{
			name:         "severity_text",
			move:         func(c *Config) { c.MoveSeverityTextField = true },
			expectedBody: map[string]any{"message": "boom", "trace_id": testTraceID, "scope_name": "cart"},
		},
		{
			name:         "trace_id",
			move:         func(c *Config) { c.MoveTraceIDField = true },
			expectedBody: map[string]any{"message": "boom", "level": "error", "scope_name": "cart"},
		},
		{
			name:         "scope_name",
			move:         func(c *Config) { c.MoveScopeNameField = true },
			expectedBody: map[string]any{"message": "boom", "level": "error", "trace_id": testTraceID},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newInferenceConfig()
			tc.move(cfg)
			processor := newProcessorWithConfig(t, cfg)

			e := newTestEntryWithTime(t, time.Now())
			e.Body = map[string]any{
				"message":    "boom",
				"level":      "error",
				"trace_id":   testTraceID,
				"scope_name": "cart",
			}
			require.NoError(t, processor.Process(context.Background(), e))

			require.Equal(t, tc.expectedBody, e.Body)
			require.Equal(t, entry.Error, e.Severity)
			require.Equal(t, decodeHexOrPanic(testTraceID), e.TraceID)
			require.Equal(t, "cart", e.ScopeName)
		})
	}
}

func TestMoveAppliesToTheSeverityFieldThatWasUsed(t *testing.T) {
	cases := []struct {
		name         string
		move         func(*Config)
		body         map[string]any
		expectedBody map[string]any
	}{
		{
			name:         "only_the_number_is_moved",
			move:         func(c *Config) { c.MoveSeverityNumberField = true },
			body:         map[string]any{"message": "boom", "severity_number": int64(17), "level": "error"},
			expectedBody: map[string]any{"message": "boom", "level": "error"},
		},
		{
			name:         "only_the_text_is_moved",
			move:         func(c *Config) { c.MoveSeverityTextField = true },
			body:         map[string]any{"message": "boom", "severity_number": int64(17), "level": "error"},
			expectedBody: map[string]any{"message": "boom", "severity_number": int64(17)},
		},
		{
			name:         "moving_the_number_does_not_remove_the_text_field",
			move:         func(c *Config) { c.MoveSeverityNumberField = true },
			body:         map[string]any{"message": "boom", "level": "error"},
			expectedBody: map[string]any{"message": "boom", "level": "error"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newInferenceConfig()
			tc.move(cfg)
			processor := newProcessorWithConfig(t, cfg)

			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.body
			require.NoError(t, processor.Process(context.Background(), e))

			require.Equal(t, tc.expectedBody, e.Body)
			require.Equal(t, entry.Error, e.Severity)
			require.Equal(t, "ERROR", e.SeverityText)
		})
	}
}

func withScopeAttributes(e *entry.Entry, scope map[string]any) {
	if e.Attributes == nil {
		e.Attributes = map[string]any{}
	}
	e.Attributes[signozstanzaentry.InternalTempScopeAttributesAttribute] = scope
}

func TestInferenceFromScopeAttributes(t *testing.T) {
	processor := newProcessorWithConfig(t, newInferenceConfig())

	scope := map[string]any{
		"level":         "error",
		"trace_id":      testTraceID,
		"scope_version": "1.4.0",
		"kept":          "value",
	}
	e := newTestEntryWithTime(t, time.Now())
	e.Body = map[string]any{"message": "boom"}
	withScopeAttributes(e, scope)
	require.NoError(t, processor.Process(context.Background(), e))

	require.Equal(t, entry.Error, e.Severity)
	require.Equal(t, "ERROR", e.SeverityText)
	require.Equal(t, decodeHexOrPanic(testTraceID), e.TraceID)
	require.Equal(t, "1.4.0", e.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute])
	require.Equal(t, map[string]any{
		"level":         "error",
		"trace_id":      testTraceID,
		"scope_version": "1.4.0",
		"kept":          "value",
	}, scope)
}

func TestSearchOrderIsBodyAttributesScopeResource(t *testing.T) {
	cases := []struct {
		name             string
		body             map[string]any
		attributes       map[string]any
		scope            map[string]any
		resource         map[string]any
		expectedSeverity entry.Severity
	}{
		{
			name:             "body_first",
			body:             map[string]any{"message": "boom", "level": "trace"},
			attributes:       map[string]any{"level": "debug"},
			scope:            map[string]any{"level": "info"},
			resource:         map[string]any{"level": "warn"},
			expectedSeverity: entry.Trace,
		},
		{
			name:             "then_attributes",
			body:             map[string]any{"message": "boom"},
			attributes:       map[string]any{"level": "debug"},
			scope:            map[string]any{"level": "info"},
			resource:         map[string]any{"level": "warn"},
			expectedSeverity: entry.Debug,
		},
		{
			name:             "then_scope",
			body:             map[string]any{"message": "boom"},
			scope:            map[string]any{"level": "info"},
			resource:         map[string]any{"level": "warn"},
			expectedSeverity: entry.Info,
		},
		{
			name:             "then_resource",
			body:             map[string]any{"message": "boom"},
			resource:         map[string]any{"level": "warn"},
			expectedSeverity: entry.Warn,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			processor := newProcessorWithConfig(t, newInferenceConfig())

			e := newTestEntryWithTime(t, time.Now())
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource
			if tc.scope != nil {
				withScopeAttributes(e, tc.scope)
			}
			require.NoError(t, processor.Process(context.Background(), e))

			require.Equal(t, tc.expectedSeverity, e.Severity)
		})
	}
}

func TestFieldNamesAreRankedPerTarget(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.MessageFields = []string{"message"}
	cfg.SeverityTextFields = []string{" Level ", "level", "", "LOG.LEVEL"}
	f := newFieldConfig(*cfg)

	require.Equal(t, []nameMatch{{target: targetMessage, rank: 0}}, f.names["message"])
	require.Equal(t, []nameMatch{{target: targetSeverityText, rank: 0}}, f.names["level"])
	require.Equal(t, []nameMatch{{target: targetSeverityText, rank: 1}}, f.names["log.level"])
	require.NotContains(t, f.names, "")
	require.NotContains(t, f.names, " level ")
}

func TestOneNameCanServeTwoTargets(t *testing.T) {
	cfg := newInferenceConfig()
	cfg.MessageFields = []string{"level"}
	cfg.SeverityTextFields = []string{"severity", "level"}
	f := newFieldConfig(*cfg)

	require.Equal(t, []nameMatch{
		{target: targetMessage, rank: 0},
		{target: targetSeverityText, rank: 1},
	}, f.names["level"])
}
