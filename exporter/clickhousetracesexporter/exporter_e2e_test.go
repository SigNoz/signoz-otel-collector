package clickhousetracesexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestExporterAttributesTypedInsertE2E(t *testing.T) {
	dsn := os.Getenv("TRACES_E2E_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("set TRACES_E2E_CLICKHOUSE_DSN to a throwaway ClickHouse to run, e.g. clickhouse://default:chtest@localhost:19010/default")
	}

	ctx := context.Background()
	adminOpts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	admin, err := clickhouse.Open(adminOpts)
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()

	for _, ddl := range []string{
		`DROP DATABASE IF EXISTS signoz_traces`,
		`CREATE DATABASE signoz_traces`,
		`CREATE DATABASE IF NOT EXISTS signoz_metadata`,
		`CREATE TABLE IF NOT EXISTS signoz_metadata.distributed_column_evolution_metadata (
			signal String,
			column_name String,
			column_type String,
			field_context String,
			field_name String,
			release_time DateTime
		) ENGINE = MergeTree ORDER BY field_name`,
		`TRUNCATE TABLE signoz_metadata.distributed_column_evolution_metadata`,
		`INSERT INTO signoz_metadata.distributed_column_evolution_metadata VALUES ('traces', 'attributes_promoted', 'JSON', 'attribute', 'user.id', now())`,
		`CREATE TABLE signoz_traces.distributed_signoz_index_v3 (
			ts_bucket_start UInt64,
			resource_fingerprint String,
			timestamp DateTime64(9),
			trace_id String,
			span_id String,
			trace_state String,
			parent_span_id String,
			flags UInt32,
			name LowCardinality(String),
			kind Int8,
			kind_string String,
			duration_nano UInt64,
			status_code Int16,
			status_message String,
			status_code_string String,
			attributes_string Map(LowCardinality(String), String),
			attributes_number Map(LowCardinality(String), Float64),
			attributes_bool Map(LowCardinality(String), Bool),
			attributes JSON(max_dynamic_paths=0) CODEC(ZSTD(1)),
			attributes_promoted JSON CODEC(ZSTD(1)),
			resources_string Map(LowCardinality(String), String),
			resource JSON(max_dynamic_paths=100) CODEC(ZSTD(1)),
			scope JSON(name String, version String, attributes JSON(max_dynamic_paths=0), max_dynamic_paths=0) CODEC(ZSTD(1)),
			events Array(String),
			links String,
			response_status_code String,
			external_http_url String,
			http_url String,
			external_http_method String,
			http_method String,
			http_host String,
			db_name String,
			db_operation String,
			has_error Bool,
			is_remote LowCardinality(String),
			inserted_at DateTime64(9)
		) ENGINE = MergeTree ORDER BY (ts_bucket_start, trace_id)`,
		`CREATE TABLE signoz_traces.distributed_signoz_error_index_v2 (
			timestamp DateTime64(9),
			errorID String,
			groupID String,
			traceID String,
			spanID String,
			serviceName String,
			exceptionType String,
			exceptionMessage String,
			exceptionStacktrace String,
			exceptionEscaped Bool,
			resourceTagsMap Map(LowCardinality(String), String)
		) ENGINE = MergeTree ORDER BY (timestamp, groupID)`,
		`CREATE TABLE signoz_traces.distributed_span_attributes_keys (
			tagKey String,
			tagType String,
			dataType String,
			isColumn Bool
		) ENGINE = MergeTree ORDER BY tagKey`,
		`CREATE TABLE signoz_traces.distributed_tag_attributes_v2 (
			unix_milli Int64,
			tag_key String,
			tag_type String,
			tag_data_type String,
			string_value String,
			number_value Nullable(Float64)
		) ENGINE = MergeTree ORDER BY tag_key`,
		`CREATE TABLE signoz_traces.distributed_traces_v3_resource (
			labels String,
			fingerprint String,
			seen_at_ts_bucket_start Int64
		) ENGINE = MergeTree ORDER BY (seen_at_ts_bucket_start, fingerprint)`,
		`CREATE TABLE signoz_traces.distributed_usage (
			tenant String,
			collector_id String,
			exporter_id String,
			timestamp DateTime,
			data String
		) ENGINE = MergeTree ORDER BY timestamp`,
	} {
		require.NoError(t, admin.Exec(ctx, ddl), ddl)
	}

	connOpts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	client, err := clickhouse.Open(connOpts)
	require.NoError(t, err)

	writerOpts := testWriterOptions()
	id := uuid.New()
	writerOpts = append(writerOpts, WithClickHouseClient(client), WithExporterID(id))
	exporterOpts := []TraceExporterOption{
		WithNewUsageCollector(id, client, zap.NewNop()),
	}
	exp, err := newExporter(&Config{}, exporter.Settings{TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()}}, writerOpts, exporterOpts)
	require.NoError(t, err)
	defer func() { require.NoError(t, exp.Shutdown(ctx)) }()

	exp.Writer.doFetchPromotedPaths()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc-a")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("otel-lib")
	ss.Scope().SetVersion("1.2.3")
	ss.Scope().Attributes().PutStr("lib.lang", "go")

	span := ss.Spans().AppendEmpty()
	span.SetName("GET /checkout")
	span.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	now := time.Now()
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(120 * time.Millisecond)))
	span.Attributes().PutStr("peer", "svc-b")
	span.Attributes().PutInt("http.status_code", 200)
	span.Attributes().PutDouble("bigfloat", 1.8446744073709552e19)
	span.Attributes().PutBool("cache.hit", true)
	span.Attributes().PutStr("user.id", "u1")
	nested := span.Attributes().PutEmptyMap("meta")
	nested.PutStr("env", "prod")
	tags := span.Attributes().PutEmptySlice("tags")
	tags.AppendEmpty().SetStr("x")
	tags.AppendEmpty().SetStr("y")
	mixed := span.Attributes().PutEmptySlice("mixed")
	mixed.AppendEmpty().SetInt(1)
	mixed.AppendEmpty().SetStr("a")
	mixed.AppendEmpty().SetEmptyMap().PutInt("k", 2)

	require.NoError(t, exp.pushTraceDataV3(ctx, td))

	var count uint64
	require.NoError(t, admin.QueryRow(ctx, `SELECT count() FROM signoz_traces.distributed_signoz_index_v3`).Scan(&count))
	assert.Equal(t, uint64(1), count)

	var attrs, attrTypes, scope, promoted string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(attributes), toJSONString(JSONAllPathsWithTypes(attributes)), toJSONString(scope), toJSONString(attributes_promoted) FROM signoz_traces.distributed_signoz_index_v3`,
	).Scan(&attrs, &attrTypes, &scope, &promoted))

	assert.Contains(t, attrs, `"bigfloat":18446744073709552000`)
	assert.JSONEq(t, `{
		"peer":"String",
		"http.status_code":"Int64",
		"bigfloat":"Float64",
		"cache.hit":"Bool",
		"user.id":"String",
		"meta.env":"String",
		"tags":"Array(Nullable(String))",
		"mixed":"Array(Dynamic)"
	}`, attrTypes)
	assert.JSONEq(t, `{"name":"otel-lib","version":"1.2.3","attributes":{"lib":{"lang":"go"}}}`, scope)
	assert.JSONEq(t, `{"user":{"id":"u1"}}`, promoted)
}
