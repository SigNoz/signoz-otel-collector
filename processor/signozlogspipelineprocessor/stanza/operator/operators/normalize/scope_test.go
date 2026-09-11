package json

import (
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/stretchr/testify/require"
)

func TestSetScope(t *testing.T) {
	cases := []struct {
		name            string
		body            any
		attributes      map[string]any
		resource        map[string]any
		scopeName       string
		expectedName    string
		expectedVersion string
	}{
		{
			name:            "scope_in_body",
			body:            map[string]any{"message": "boom", "scope.name": "checkout", "scope.version": "1.4.0"},
			expectedName:    "checkout",
			expectedVersion: "1.4.0",
		},
		{
			name:            "field_names_are_case_insensitive",
			body:            map[string]any{"scopeName": "checkout", "scopeVersion": "1.4.0"},
			expectedName:    "checkout",
			expectedVersion: "1.4.0",
		},
		{
			name: "logger_fields_are_not_scope_names_by_default",
			body: map[string]any{"logger": "cart", "logger_name": "com.signoz.checkout.CartService"},
		},
		{
			name:         "otel_field_wins_over_a_logger_field",
			body:         map[string]any{"logger_name": "cart", "scope_name": "checkout"},
			expectedName: "checkout",
		},
		{
			name:         "scope_in_attributes",
			body:         map[string]any{"message": "boom"},
			attributes:   map[string]any{"scope_name": "checkout"},
			expectedName: "checkout",
		},
		{
			name:         "scope_in_resource",
			body:         map[string]any{"message": "boom"},
			resource:     map[string]any{"scope_name": "cart"},
			expectedName: "cart",
		},
		{
			name:         "body_wins_over_attributes",
			body:         map[string]any{"scope_name": "checkout"},
			attributes:   map[string]any{"scope_name": "payments"},
			expectedName: "checkout",
		},
		{
			name: "blank_scope_name_is_ignored",
			body: map[string]any{"scope_name": "   "},
		},
		{
			name: "non_string_scope_name_is_ignored",
			body: map[string]any{"scope_name": int64(3)},
		},
		{
			name:         "existing_scope_name_is_kept",
			body:         map[string]any{"scope_name": "checkout"},
			scopeName:    "already-set",
			expectedName: "already-set",
		},
		{
			name:            "version_is_read_even_when_the_name_is_already_set",
			body:            map[string]any{"scope_version": "2.0.0"},
			scopeName:       "already-set",
			expectedName:    "already-set",
			expectedVersion: "2.0.0",
		},
		{
			name: "non_map_body_is_left_alone",
			body: "logger=cart",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry.New()
			e.Body = tc.body
			e.Attributes = tc.attributes
			e.Resource = tc.resource
			e.ScopeName = tc.scopeName

			defaultFields.infer(e)

			require.Equal(t, tc.expectedName, e.ScopeName)

			version, exists := e.Attributes[signozstanzaentry.InternalTempScopeVersionAttribute]
			if tc.expectedVersion == "" {
				require.False(t, exists, "no scope version should have been recorded")
				return
			}
			require.True(t, exists, "scope version should have been recorded")
			require.Equal(t, tc.expectedVersion, version)
		})
	}
}
