package json

import (
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/stretchr/testify/require"
)

func TestSetSeverity(t *testing.T) {
	cases := []struct {
		name             string
		body             any
		attributes       map[string]any
		resource         map[string]any
		severity         entry.Severity
		severityText     string
		expectedSeverity entry.Severity
		expectedText     string
	}{
		{
			name:             "level_in_body",
			body:             map[string]any{"message": "boom", "level": "error"},
			expectedSeverity: entry.Error,
			expectedText:     "ERROR",
		},
		{
			name:             "level_name_is_canonicalized",
			body:             map[string]any{"level": "Warning"},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "level_name_is_trimmed",
			body:             map[string]any{"level": " INFO "},
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "level_field_name_is_case_insensitive",
			body:             map[string]any{"LEVEL": "debug"},
			expectedSeverity: entry.Debug,
			expectedText:     "DEBUG",
		},
		{
			name:             "syslog_level_name",
			body:             map[string]any{"severity": "critical"},
			expectedSeverity: entry.Error2,
			expectedText:     "ERROR",
		},
		{
			name:             "syslog_notice",
			body:             map[string]any{"severity": "notice"},
			expectedSeverity: entry.Info2,
			expectedText:     "INFO",
		},
		{
			name:             "otel_severity_text_with_a_group_suffix",
			body:             map[string]any{"severity_text": "WARN3"},
			expectedSeverity: entry.Warn3,
			expectedText:     "WARN",
		},
		{
			name:             "java_util_logging_level_name",
			body:             map[string]any{"level": "SEVERE"},
			expectedSeverity: entry.Error,
			expectedText:     "ERROR",
		},
		{
			name:             "python_levelname",
			body:             map[string]any{"levelname": "INFO"},
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "flattened_ecs_log_level",
			body:             map[string]any{"log.level": "warn"},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "severity_number",
			body:             map[string]any{"severity_number": int64(17)},
			expectedSeverity: entry.Error,
			expectedText:     "ERROR",
		},
		{
			name:             "severity_number_as_float",
			body:             map[string]any{"severityNumber": float64(9)},
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "severity_number_as_string",
			body:             map[string]any{"severity_number": "5"},
			expectedSeverity: entry.Debug,
			expectedText:     "DEBUG",
		},
		{
			name:             "severity_number_as_otlp_enum_name",
			body:             map[string]any{"severityNumber": "SEVERITY_NUMBER_WARN"},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "severity_number_out_of_range_is_ignored",
			body:             map[string]any{"severity_number": int64(30)},
			expectedSeverity: entry.Default,
		},
		{
			name:             "number_and_text_are_read_independently",
			body:             map[string]any{"severity_number": int64(13), "level": "info"},
			expectedSeverity: entry.Warn,
			expectedText:     "INFO",
		},
		{
			name:             "text_fills_in_the_number_when_there_is_no_number_field",
			body:             map[string]any{"level": "warn"},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "number_fills_in_the_text_when_there_is_no_text_field",
			body:             map[string]any{"severity_number": int64(13)},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "more_specific_field_wins",
			body:             map[string]any{"severity_text": "error", "level": "info"},
			expectedSeverity: entry.Error,
			expectedText:     "ERROR",
		},
		{
			name:             "unknown_level_name_is_recorded_as_the_log_wrote_it",
			body:             map[string]any{"severity": "loud", "level": "error"},
			expectedSeverity: entry.Default,
			expectedText:     "loud",
		},
		{
			name:             "a_long_value_is_recorded_in_full",
			body:             map[string]any{"level": strings.Repeat("x", 200)},
			expectedSeverity: entry.Default,
			expectedText:     strings.Repeat("x", 200),
		},
		{
			name:             "falls_back_to_next_field_when_value_is_not_a_string",
			body:             map[string]any{"severity": map[string]any{"code": 3}, "level": "error"},
			expectedSeverity: entry.Error,
			expectedText:     "ERROR",
		},
		{
			name:             "numeric_level_is_ignored",
			body:             map[string]any{"level": int64(30)},
			expectedSeverity: entry.Default,
		},
		{
			name:             "unknown_level_name_does_not_name_a_number",
			body:             map[string]any{"level": "chatty"},
			expectedSeverity: entry.Default,
			expectedText:     "chatty",
		},
		{
			name:             "a_number_still_comes_from_the_number_field_when_the_text_is_unknown",
			body:             map[string]any{"level": "chatty", "severity_number": int64(17)},
			expectedSeverity: entry.Error,
			expectedText:     "chatty",
		},
		{
			name:             "text_log_body_has_nothing_to_read",
			body:             map[string]any{"message": "something failed"},
			expectedSeverity: entry.Default,
		},
		{
			name:             "level_in_attributes",
			body:             map[string]any{"message": "boom"},
			attributes:       map[string]any{"level": "fatal"},
			expectedSeverity: entry.Fatal,
			expectedText:     "FATAL",
		},
		{
			name:             "level_in_resource",
			body:             map[string]any{"message": "boom"},
			resource:         map[string]any{"level": "trace"},
			expectedSeverity: entry.Trace,
			expectedText:     "TRACE",
		},
		{
			name:             "body_wins_over_attributes_and_resource",
			body:             map[string]any{"level": "info"},
			attributes:       map[string]any{"level": "error"},
			resource:         map[string]any{"level": "fatal"},
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "each_half_is_read_from_wherever_it_is_found",
			body:             map[string]any{"level": "info"},
			attributes:       map[string]any{"severity_number": int64(17)},
			expectedSeverity: entry.Error,
			expectedText:     "INFO",
		},
		{
			name:             "attributes_win_over_resource",
			body:             map[string]any{"message": "boom"},
			attributes:       map[string]any{"level": "warn"},
			resource:         map[string]any{"level": "error"},
			expectedSeverity: entry.Warn,
			expectedText:     "WARN",
		},
		{
			name:             "existing_severity_text_is_kept",
			body:             map[string]any{"level": "error"},
			severityText:     "whatever the producer said",
			expectedSeverity: entry.Default,
			expectedText:     "whatever the producer said",
		},
		{
			name:             "existing_severity_text_is_kept_over_a_level_the_operator_does_not_know",
			body:             map[string]any{"level": "AUDIT"},
			severityText:     "whatever the producer said",
			expectedSeverity: entry.Default,
			expectedText:     "whatever the producer said",
		},
		{
			name:             "an_unset_text_takes_a_level_the_operator_does_not_know_over_the_number_name",
			body:             map[string]any{"level": "AUDIT"},
			severity:         entry.Info,
			expectedSeverity: entry.Info,
			expectedText:     "AUDIT",
		},
		{
			name:             "existing_severity_number_is_kept_while_the_text_is_still_read",
			body:             map[string]any{"level": "error"},
			severity:         entry.Info,
			expectedSeverity: entry.Info,
			expectedText:     "ERROR",
		},
		{
			name:             "existing_severity_number_names_its_own_text_when_the_log_has_none",
			body:             map[string]any{"message": "boom"},
			severity:         entry.Info,
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "existing_severity_text_is_kept_while_the_number_is_still_read",
			body:             map[string]any{"severity_number": int64(17)},
			severityText:     "warning",
			expectedSeverity: entry.Error,
			expectedText:     "warning",
		},
		{
			name:             "existing_severity_number_is_not_replaced_by_a_severity_number_field",
			body:             map[string]any{"severity_number": int64(17)},
			severity:         entry.Info,
			expectedSeverity: entry.Info,
			expectedText:     "INFO",
		},
		{
			name:             "existing_severity_text_fills_in_the_number",
			body:             map[string]any{"level": "error"},
			severityText:     "warning",
			expectedSeverity: entry.Warn,
			expectedText:     "warning",
		},
		{
			name:             "non_map_body_is_left_alone",
			body:             "some text log",
			expectedSeverity: entry.Default,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry.New()
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource
			e.Severity = tc.severity
			e.SeverityText = tc.severityText

			defaultFields.infer(e)

			require.Equal(t, tc.expectedSeverity, e.Severity)
			require.Equal(t, tc.expectedText, e.SeverityText)
		})
	}
}

func TestSetSeverityIsDeterministicForCaseCollisions(t *testing.T) {
	for range 100 {
		e := entry.New()
		e.Body = map[string]any{"Level": "error", "level": "info", "LEVEL": "warn"}

		defaultFields.infer(e)

		require.Equal(t, entry.Warn, e.Severity)
		require.Equal(t, "WARN", e.SeverityText)
	}
}
