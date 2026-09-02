package clickhousetracesexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

func TestSpanFieldRows(t *testing.T) {
	tests := []struct {
		name     string
		span     *SpanV3
		wantRows []spanFieldRow
		skipRows []spanFieldRow
	}{
		{
			name: "intrinsic columns are written even when zero",
			span: &SpanV3{Name: "GET /users/{id}", Kind: 0, SpanKind: "Unspecified", StatusCode: 0, StatusCodeString: "Unset"},
			wantRows: []spanFieldRow{
				{key: "name", dataType: utils.FieldDataTypeString, stringValue: "GET /users/{id}"},
				{key: "kind_string", dataType: utils.FieldDataTypeString, stringValue: "Unspecified"},
				{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 0},
				{key: "status_code_string", dataType: utils.FieldDataTypeString, stringValue: "Unset"},
				{key: "status_code", dataType: utils.FieldDataTypeFloat64, numberValue: 0},
			},
		},
		{
			name: "calculated columns are written when set and flagged as calculated",
			span: &SpanV3{HttpMethod: "GET", HttpHost: "api.example.com", HttpUrl: "https://api.example.com/users/42", ResponseStatusCode: "200", IsRemote: "false"},
			wantRows: []spanFieldRow{
				{key: "http_method", dataType: utils.FieldDataTypeString, stringValue: "GET", calculated: true},
				{key: "http_host", dataType: utils.FieldDataTypeString, stringValue: "api.example.com", calculated: true},
				{key: "http_url", dataType: utils.FieldDataTypeString, stringValue: "https://api.example.com/users/42", calculated: true},
				{key: "response_status_code", dataType: utils.FieldDataTypeString, stringValue: "200", calculated: true},
				{key: "is_remote", dataType: utils.FieldDataTypeString, stringValue: "false", calculated: true},
			},
		},
		{
			name: "empty calculated columns are not written",
			span: &SpanV3{HttpMethod: "GET"},
			skipRows: []spanFieldRow{
				{key: "db_name", dataType: utils.FieldDataTypeString, stringValue: "", calculated: true},
				{key: "db_operation", dataType: utils.FieldDataTypeString, stringValue: "", calculated: true},
				{key: "external_http_method", dataType: utils.FieldDataTypeString, stringValue: "", calculated: true},
				{key: "external_http_url", dataType: utils.FieldDataTypeString, stringValue: "", calculated: true},
			},
		},
		{
			name: "has_error is written as a bool key without a value",
			span: &SpanV3{HasError: true},
			wantRows: []spanFieldRow{
				{key: "has_error", dataType: utils.FieldDataTypeBool, calculated: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := spanFieldRows(tt.span)

			for _, want := range tt.wantRows {
				assert.Contains(t, rows, want, "expected a row for %s", want.key)
			}
			for _, skip := range tt.skipRows {
				assert.NotContains(t, rows, skip, "expected no row for empty %s", skip.key)
			}
		})
	}
}

func TestSpanFieldRowDedupeKey(t *testing.T) {
	sameValueDifferentColumns := []spanFieldRow{
		{key: "name", dataType: utils.FieldDataTypeString, stringValue: "Server"},
		{key: "kind_string", dataType: utils.FieldDataTypeString, stringValue: "Server"},
	}
	sameColumnDifferentValues := []spanFieldRow{
		{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 1},
		{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 2},
	}

	assert.NotEqual(t, sameValueDifferentColumns[0].dedupeKey(), sameValueDifferentColumns[1].dedupeKey(), "a value shared by two columns must yield two rows")
	assert.NotEqual(t, sameColumnDifferentValues[0].dedupeKey(), sameColumnDifferentValues[1].dedupeKey(), "two values of one column must yield two rows")
}
