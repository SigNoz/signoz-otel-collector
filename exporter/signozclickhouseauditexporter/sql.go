package signozclickhouseauditexporter

const (
	databaseName                   = "signoz_audit"
	distributedLogsTable           = "distributed_logs"
	distributedLogsResource        = "distributed_logs_resource"
	distributedLogsAttributeKeys   = "distributed_logs_attribute_keys"
	distributedLogsResourceKeys    = "distributed_logs_resource_keys"
	distributedTagAttributes       = "distributed_tag_attributes"
	distributedLogsResourceSeconds = 1800

	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s.%s (
		ts_bucket_start,
		resource_fingerprint,
		timestamp,
		observed_timestamp,
		id,
		trace_id,
		span_id,
		trace_flags,
		severity_text,
		severity_number,
		body,
		scope_name,
		scope_version,
		scope_string,
		attributes_string,
		attributes_number,
		attributes_bool,
		resource,
		event_name
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?
	)`

	// language=ClickHouse SQL
	insertLogsResourceSQLTemplate = `INSERT INTO %s.%s (
		labels,
		fingerprint,
		seen_at_ts_bucket_start
	) VALUES (?, ?, ?)`
)
