package clickhousetracesexporter

import (
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

// spanFieldDedupeKey identifies a row within a write batch. It is a struct so
// that map lookups do not allocate.
type spanFieldDedupeKey struct {
	key         string
	dataType    utils.FieldDataType
	stringValue string
	numberValue float64
}

func (r spanFieldRow) dedupeKey() spanFieldDedupeKey {
	return spanFieldDedupeKey{key: r.key, dataType: r.dataType, stringValue: r.stringValue, numberValue: r.numberValue}
}

// spanFieldRows appends the rows for one span's top-level columns to rows,
// which the caller reuses across spans. String rows with an empty value are
// omitted. Numeric rows are always returned since 0 is a valid kind and
// status code.
func spanFieldRows(span *SpanV3, rows []spanFieldRow) []spanFieldRow {
	rows = appendSpanFieldString(rows, "name", span.Name, false)
	rows = appendSpanFieldString(rows, "kind_string", span.SpanKind, false)
	rows = append(rows, spanFieldRow{key: "kind", dataType: utils.FieldDataTypeFloat64, numberValue: float64(span.Kind)})
	rows = appendSpanFieldString(rows, "status_code_string", span.StatusCodeString, false)
	rows = append(rows, spanFieldRow{key: "status_code", dataType: utils.FieldDataTypeFloat64, numberValue: float64(span.StatusCode)})
	rows = appendSpanFieldString(rows, "http_method", span.HttpMethod, true)
	rows = appendSpanFieldString(rows, "http_host", span.HttpHost, true)
	rows = appendSpanFieldString(rows, "http_url", span.HttpUrl, true)
	rows = appendSpanFieldString(rows, "response_status_code", span.ResponseStatusCode, true)
	rows = appendSpanFieldString(rows, "db_name", span.DBName, true)
	rows = appendSpanFieldString(rows, "db_operation", span.DBOperation, true)
	rows = appendSpanFieldString(rows, "external_http_method", span.ExternalHttpMethod, true)
	rows = appendSpanFieldString(rows, "external_http_url", span.ExternalHttpUrl, true)
	rows = appendSpanFieldString(rows, "is_remote", span.IsRemote, true)
	rows = append(rows, spanFieldRow{key: "has_error", dataType: utils.FieldDataTypeBool, calculated: true})
	return rows
}

func appendSpanFieldString(rows []spanFieldRow, key, value string, calculated bool) []spanFieldRow {
	if value == "" {
		return rows
	}
	return append(rows, spanFieldRow{key: key, dataType: utils.FieldDataTypeString, stringValue: value, calculated: calculated})
}
