package signozclickhouseauditexporter

const (
	database                     = "signoz_audit"
	distributedLogsTable         = "distributed_logs"
	distributedLogsResourceTable = "distributed_logs_resource"

	// language=ClickHouse SQL
	insertLogsSQL = `INSERT INTO ` + database + `.` + distributedLogsTable + ` (
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
		scope,
		attributes,
		resource,
		event_name
	)`

	// language=ClickHouse SQL
	insertLogsResourceSQL = `INSERT INTO ` + database + `.` + distributedLogsResourceTable + ` (
		labels,
		fingerprint,
		seen_at_ts_bucket_start
	)`
)
