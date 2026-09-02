package clickhousetracesexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

func TestSpanFieldRows(t *testing.T) {
	span := &SpanV3{
		Name:               "GET /users/{id}",
		Kind:               2,
		SpanKind:           "Server",
		StatusCode:         0,
		StatusCodeString:   "Unset",
		HttpMethod:         "GET",
		HttpHost:           "api.example.com",
		HttpUrl:            "https://api.example.com/users/42",
		ResponseStatusCode: "200",
		DBName:             "",
		DBOperation:        "",
		ExternalHttpMethod: "",
		ExternalHttpUrl:    "",
		IsRemote:           "false",
		HasError:           false,
	}

	rows := spanFieldRows(span)

	byKey := make(map[string]spanFieldRow, len(rows))
	for _, row := range rows {
		_, dup := byKey[row.key]
		require.False(t, dup, "row for %q returned twice", row.key)
		byKey[row.key] = row
	}

	// intrinsic columns are always returned, including numeric zeros
	assert.Equal(t, spanFieldRow{key: "name", dataType: utils.FieldDataTypeString, stringValue: "GET /users/{id}"}, byKey["name"])
	assert.Equal(t, spanFieldRow{key: "kind_string", dataType: utils.FieldDataTypeString, stringValue: "Server"}, byKey["kind_string"])
	assert.Equal(t, spanFieldRow{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 2}, byKey["kind"])
	assert.Equal(t, spanFieldRow{key: "status_code_string", dataType: utils.FieldDataTypeString, stringValue: "Unset"}, byKey["status_code_string"])
	assert.Equal(t, spanFieldRow{key: "status_code", dataType: utils.FieldDataTypeFloat64, numberValue: 0}, byKey["status_code"])

	// calculated columns are written when set and flagged so the writer applies attribute limits
	assert.Equal(t, spanFieldRow{key: "http_method", dataType: utils.FieldDataTypeString, stringValue: "GET", calculated: true}, byKey["http_method"])
	assert.Equal(t, spanFieldRow{key: "http_host", dataType: utils.FieldDataTypeString, stringValue: "api.example.com", calculated: true}, byKey["http_host"])
	assert.Equal(t, spanFieldRow{key: "http_url", dataType: utils.FieldDataTypeString, stringValue: "https://api.example.com/users/42", calculated: true}, byKey["http_url"])
	assert.Equal(t, spanFieldRow{key: "response_status_code", dataType: utils.FieldDataTypeString, stringValue: "200", calculated: true}, byKey["response_status_code"])
	assert.Equal(t, spanFieldRow{key: "is_remote", dataType: utils.FieldDataTypeString, stringValue: "false", calculated: true}, byKey["is_remote"])

	// bool columns record the key and type only, like bool span attributes
	assert.Equal(t, spanFieldRow{key: "has_error", dataType: utils.FieldDataTypeBool, calculated: true}, byKey["has_error"])

	// empty calculated strings are not suggestions
	for _, key := range []string{"db_name", "db_operation", "external_http_method", "external_http_url"} {
		_, ok := byKey[key]
		assert.False(t, ok, "expected no row for empty %q", key)
	}
}

func TestSpanFieldRowDedupeKeyIsPerColumn(t *testing.T) {
	// the same value under two columns must not collapse into one row
	name := spanFieldRow{key: "name", dataType: utils.FieldDataTypeString, stringValue: "Server"}
	kind := spanFieldRow{key: "kind_string", dataType: utils.FieldDataTypeString, stringValue: "Server"}
	assert.NotEqual(t, name.dedupeKey(), kind.dedupeKey())

	// and the same column with two values yields two rows
	assert.NotEqual(t,
		spanFieldRow{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 1}.dedupeKey(),
		spanFieldRow{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: 2}.dedupeKey(),
	)
}
