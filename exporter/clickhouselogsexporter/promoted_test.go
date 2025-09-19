package clickhouselogsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestPromotedPathSeparation(t *testing.T) {
	testCases := []struct {
		name             string
		body             map[string]interface{}
		promotedPaths    map[string]struct{}
		expectedBody     map[string]interface{}
		expectedPromoted map[string]interface{}
	}{
		{
			name: "simple_literal_key_match",
			body: map[string]interface{}{
				"message": "test log",
				"level":   "info",
				"user.id": "123",
			},
			promotedPaths: map[string]struct{}{
				"user.id": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"level":   "info",
			},
			expectedPromoted: map[string]interface{}{
				"user.id": "123",
			},
		},
		{
			name: "nested_path_extraction",
			body: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"id":    "123",
					"name":  "john",
					"email": "john@example.com",
				},
			},
			promotedPaths: map[string]struct{}{
				"user.id":   {},
				"user.name": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"email": "john@example.com",
				},
			},
			expectedPromoted: map[string]interface{}{
				"user.id":   "123",
				"user.name": "john",
			},
		},
		{
			name: "ambiguous_dot_notation_literal_preference",
			body: map[string]interface{}{
				"message": "test log",
				"a.b.c":   "literal_value",
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "nested_value",
					},
				},
			},
			promotedPaths: map[string]struct{}{
				"a.b.c": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "nested_value",
					},
				},
			},
			expectedPromoted: map[string]interface{}{
				"a.b.c": "literal_value",
			},
		},
		{
			name: "multiple_siblings_same_parent",
			body: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"id":    "123",
					"name":  "john",
					"email": "john@example.com",
					"address": map[string]interface{}{
						"street": "123 Main St",
						"city":   "New York",
					},
				},
			},
			promotedPaths: map[string]struct{}{
				"user.id":           {},
				"user.name":         {},
				"user.address.city": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"email": "john@example.com",
					"address": map[string]interface{}{
						"street": "123 Main St",
					},
				},
			},
			expectedPromoted: map[string]interface{}{
				"user.id":           "123",
				"user.name":         "john",
				"user.address.city": "New York",
			},
		},
		{
			name: "multiple_siblings_same_parent_with_dot_notation",
			body: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"id":             "123",
					"name":           "john",
					"email":          "john@example.com",
					"address.street": "123 Main St",
					"address.city":   "New York",
				},
			},
			promotedPaths: map[string]struct{}{
				"user.id":           {},
				"user.name":         {},
				"user.address.city": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"email": "john@example.com",
					"address": map[string]interface{}{
						"street": "123 Main St",
					},
				},
			},
			expectedPromoted: map[string]interface{}{
				"user.id":           "123",
				"user.name":         "john",
				"user.address.city": "New York",
			},
		},
		{
			name: "deeply_nested_paths",
			body: map[string]interface{}{
				"message": "test log",
				"request": map[string]interface{}{
					"headers": map[string]interface{}{
						"authorization": "Bearer token123",
						"content-type":  "application/json",
					},
					"body": map[string]interface{}{
						"user": map[string]interface{}{
							"profile": map[string]interface{}{
								"settings": map[string]interface{}{
									"theme": "dark",
								},
							},
						},
					},
				},
			},
			promotedPaths: map[string]struct{}{
				"request.headers.authorization":            {},
				"request.body.user.profile.settings.theme": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"request": map[string]interface{}{
					"headers": map[string]interface{}{
						"content-type": "application/json",
					},
				},
			},
			expectedPromoted: map[string]interface{}{
				"request.headers.authorization":            "Bearer token123",
				"request.body.user.profile.settings.theme": "dark",
			},
		},
		{
			name: "no_promoted_paths",
			body: map[string]interface{}{
				"message": "test log",
				"level":   "info",
				"user": map[string]interface{}{
					"id": "123",
				},
			},
			promotedPaths: map[string]struct{}{},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"level":   "info",
				"user": map[string]interface{}{
					"id": "123",
				},
			},
			expectedPromoted: map[string]interface{}{},
		},
		{
			name: "non_existent_paths",
			body: map[string]interface{}{
				"message": "test log",
				"level":   "info",
			},
			promotedPaths: map[string]struct{}{
				"non.existent.path": {},
				"another.missing":   {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
				"level":   "info",
			},
			expectedPromoted: map[string]interface{}{},
		},
		{
			name: "mixed_literal_and_nested",
			body: map[string]interface{}{
				"message": "test log",
				"a.b.c":   "literal",
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"d": "nested",
					},
				},
			},
			promotedPaths: map[string]struct{}{
				"a.b.c": {},
				"a.b.d": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
			},
			expectedPromoted: map[string]interface{}{
				"a.b.c": "literal",
				"a.b.d": "nested",
			},
		},
		{
			name: "empty_maps_after_extraction",
			body: map[string]interface{}{
				"message": "test log",
				"user": map[string]interface{}{
					"id": "123",
				},
			},
			promotedPaths: map[string]struct{}{
				"user.id": {},
			},
			expectedBody: map[string]interface{}{
				"message": "test log",
			},
			expectedPromoted: map[string]interface{}{
				"user.id": "123",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create pcommon.Value from test body
			body := pcommon.NewValueMap()
			populateMapFromInterface(body.Map(), tc.body)

			// Extract promoted paths
			promoted := buildPromotedAndPruneBody(body, tc.promotedPaths)

			// Convert results back to interface{} for comparison
			actualBody := convertMapToInterface(body.Map())
			actualPromoted := convertMapToInterface(promoted.Map())

			// Assertions
			assert.Equal(t, tc.expectedBody, actualBody, "Body should match expected after extraction")
			assert.Equal(t, tc.expectedPromoted, actualPromoted, "Promoted should match expected")
		})
	}
}

// Helper function to populate pcommon.Map from interface{}
func populateMapFromInterface(m pcommon.Map, data map[string]interface{}) {
	for k, v := range data {
		switch val := v.(type) {
		case string:
			m.PutStr(k, val)
		case int:
			m.PutInt(k, int64(val))
		case float64:
			m.PutDouble(k, val)
		case bool:
			m.PutBool(k, val)
		case map[string]interface{}:
			nested := m.PutEmptyMap(k)
			populateMapFromInterface(nested, val)
		}
	}
}

// Helper function to convert pcommon.Map to interface{}
func convertMapToInterface(m pcommon.Map) map[string]interface{} {
	result := make(map[string]interface{})
	m.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			result[k] = v.Str()
		case pcommon.ValueTypeInt:
			result[k] = v.Int()
		case pcommon.ValueTypeDouble:
			result[k] = v.Double()
		case pcommon.ValueTypeBool:
			result[k] = v.Bool()
		case pcommon.ValueTypeMap:
			result[k] = convertMapToInterface(v.Map())
		default:
			result[k] = v.AsString()
		}
		return true
	})
	return result
}
