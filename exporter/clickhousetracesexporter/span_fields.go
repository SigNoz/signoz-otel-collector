package clickhousetracesexporter

import (
	"strconv"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

// spanFieldRow is a tag_attributes_v2 row for a top-level span column, written
// with tag_type=spanfield.
type spanFieldRow struct {
	key      string
	dataType utils.FieldDataType
	// stringValue is set for string rows and numberValue for float64 rows;
	// bool rows carry neither, like bool span attributes.
	stringValue string
	numberValue float64
	// calculated marks the columns derived from span attributes. They are
	// subject to the attribute value length limit and the high-cardinality
	// skip list; the intrinsic columns are always written.
	calculated bool
}

// spanFieldRows omits string rows with an empty value. Numeric rows are always
// returned since 0 is a valid kind and status code.
func spanFieldRows(span *SpanV3) []spanFieldRow {
	candidates := []spanFieldRow{
		{key: "name", dataType: utils.FieldDataTypeString, stringValue: span.Name},
		{key: "kind_string", dataType: utils.FieldDataTypeString, stringValue: span.SpanKind},
		{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: float64(span.Kind)},
		{key: "status_code_string", dataType: utils.FieldDataTypeString, stringValue: span.StatusCodeString},
		{key: "status_code", dataType: utils.FieldDataTypeFloat64, numberValue: float64(span.StatusCode)},
		{key: "http_method", dataType: utils.FieldDataTypeString, stringValue: span.HttpMethod, calculated: true},
		{key: "http_host", dataType: utils.FieldDataTypeString, stringValue: span.HttpHost, calculated: true},
		{key: "http_url", dataType: utils.FieldDataTypeString, stringValue: span.HttpUrl, calculated: true},
		{key: "response_status_code", dataType: utils.FieldDataTypeString, stringValue: span.ResponseStatusCode, calculated: true},
		{key: "db_name", dataType: utils.FieldDataTypeString, stringValue: span.DBName, calculated: true},
		{key: "db_operation", dataType: utils.FieldDataTypeString, stringValue: span.DBOperation, calculated: true},
		{key: "external_http_method", dataType: utils.FieldDataTypeString, stringValue: span.ExternalHttpMethod, calculated: true},
		{key: "external_http_url", dataType: utils.FieldDataTypeString, stringValue: span.ExternalHttpUrl, calculated: true},
		{key: "is_remote", dataType: utils.FieldDataTypeString, stringValue: span.IsRemote, calculated: true},
		{key: "has_error", dataType: utils.FieldDataTypeBool, calculated: true},
	}

	rows := make([]spanFieldRow, 0, len(candidates))
	for _, row := range candidates {
		if row.dataType == utils.FieldDataTypeString && row.stringValue == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func (r spanFieldRow) dedupeKey() string {
	return r.key + "\x00" + string(r.dataType) + "\x00" + r.stringValue + "\x00" + strconv.FormatFloat(r.numberValue, 'f', -1, 64)
}
