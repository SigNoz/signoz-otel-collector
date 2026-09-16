package clickhouselogsexporter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

func TestExporterBodyJSONTypedInsertE2E(t *testing.T) {
	dsn := os.Getenv("LOGS_E2E_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("set LOGS_E2E_CLICKHOUSE_DSN to a throwaway ClickHouse to run, e.g. clickhouse://default:chtest@localhost:19010/default")
	}

	ctx := context.Background()
	adminOpts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	admin, err := clickhouse.Open(adminOpts)
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()

	for _, ddl := range []string{
		`DROP DATABASE IF EXISTS signoz_logs`,
		`DROP DATABASE IF EXISTS signoz_metadata`,
		`CREATE DATABASE signoz_logs`,
		`CREATE DATABASE signoz_metadata`,
		`CREATE TABLE signoz_logs.distributed_logs_v2 (
			ts_bucket_start UInt64,
			resource_fingerprint String,
			timestamp UInt64,
			observed_timestamp UInt64,
			id String,
			trace_id String,
			span_id String,
			trace_flags UInt32,
			severity_text LowCardinality(String),
			severity_number UInt8,
			body String,
			body_v2 JSON(message String, max_dynamic_paths=0) CODEC(ZSTD(1)),
			body_promoted JSON CODEC(ZSTD(1)),
			attributes_string Map(LowCardinality(String), String),
			attributes_number Map(LowCardinality(String), Float64),
			attributes_bool Map(LowCardinality(String), Bool),
			resources_string Map(LowCardinality(String), String),
			resource JSON(max_dynamic_paths=100) CODEC(ZSTD(1)),
			scope_name String,
			scope_version String,
			scope_string Map(LowCardinality(String), String),
			inserted_at DateTime64(9)
		) ENGINE = MergeTree ORDER BY (ts_bucket_start, id)`,
		`CREATE TABLE signoz_logs.distributed_logs_v2_resource (
			labels String,
			fingerprint String,
			seen_at_ts_bucket_start Int64
		) ENGINE = MergeTree ORDER BY (seen_at_ts_bucket_start, fingerprint)`,
		`CREATE TABLE signoz_logs.distributed_logs_attribute_keys (
			name String,
			datatype String
		) ENGINE = MergeTree ORDER BY name`,
		`CREATE TABLE signoz_logs.distributed_logs_resource_keys (
			name String,
			datatype String
		) ENGINE = MergeTree ORDER BY name`,
		`CREATE TABLE signoz_logs.distributed_tag_attributes_v2 (
			unix_milli Int64,
			tag_key String,
			tag_type String,
			tag_data_type String,
			string_value String,
			number_value Nullable(Float64)
		) ENGINE = MergeTree ORDER BY tag_key`,
		`CREATE TABLE signoz_logs.distributed_usage (
			tenant String,
			collector_id String,
			exporter_id String,
			timestamp DateTime,
			data String
		) ENGINE = MergeTree ORDER BY timestamp`,
		`CREATE TABLE signoz_metadata.distributed_column_evolution_metadata (
			signal String,
			column_name String,
			column_type String,
			field_context String,
			field_name String,
			release_time DateTime
		) ENGINE = MergeTree ORDER BY field_name`,
		`INSERT INTO signoz_metadata.distributed_column_evolution_metadata VALUES ('logs', 'body_promoted', 'JSON', 'body', 'user.id', now())`,
	} {
		require.NoError(t, admin.Exec(ctx, ddl), ddl)
	}

	cfg := &Config{
		DSN:                       dsn,
		BodyJSONEnabled:           true,
		PromotedPathsSyncInterval: utils.ToPointer(5 * time.Minute),
		LogLevelConcurrency:       utils.ToPointer(1),
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: time.Minute,
			MaxDistinctValues: 25000,
		},
	}
	require.NoError(t, cfg.Validate())

	client, err := newClickhouseClient(zap.NewNop(), cfg)
	require.NoError(t, err)

	opts := testOptions(t)
	opts = append(opts, WithClickHouseClient(client), WithNewUsageCollector(uuid.New(), client))
	exp, err := newExporter(exporter.Settings{}, cfg, opts...)
	require.NoError(t, err)
	require.NoError(t, exp.Start(ctx, nil))
	defer func() { require.NoError(t, exp.Shutdown(ctx)) }()

	now := pcommon.NewTimestampFromTime(time.Now())
	ld := plog.NewLogs()
	sl := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	appendRecord := func(fill func(body pcommon.Value)) {
		rec := sl.LogRecords().AppendEmpty()
		rec.SetTimestamp(now)
		rec.SetObservedTimestamp(now)
		fill(rec.Body())
	}

	appendRecord(func(body pcommon.Value) {
		m := body.SetEmptyMap()
		m.PutStr("message", "habroker metrics")
		m.PutEmptyMap("metrics").PutDouble("cgroup_memory_limit_bytes", 1.8446744073709552e19)
		m.PutInt("count", 42)
		m.PutDouble("pi", 3.14)
		m.PutBool("flag", true)
	})
	appendRecord(func(body pcommon.Value) {
		m := body.SetEmptyMap()
		m.PutStr("message", "arrays")
		tags := m.PutEmptySlice("tags")
		tags.AppendEmpty().SetStr("x")
		tags.AppendEmpty().SetStr("y")
		nums := m.PutEmptySlice("nums")
		nums.AppendEmpty().SetInt(1)
		nums.AppendEmpty()
		nums.AppendEmpty().SetInt(2)
		mixed := m.PutEmptySlice("mixed")
		mixed.AppendEmpty().SetInt(1)
		mixed.AppendEmpty().SetStr("a")
		mixed.AppendEmpty().SetEmptyMap().PutInt("x", 2)
		mixed.AppendEmpty().SetEmptySlice().AppendEmpty().SetInt(3)
		objs := m.PutEmptySlice("objs")
		objs.AppendEmpty().SetEmptyMap().PutInt("k", 1)
		objs.AppendEmpty().SetEmptyMap().PutInt("k", 2)
	})
	appendRecord(func(body pcommon.Value) {
		body.SetStr("plain text line")
	})
	appendRecord(func(body pcommon.Value) {
		body.SetEmptyMap().PutInt("message", 123)
	})
	appendRecord(func(body pcommon.Value) {
		m := body.SetEmptyMap()
		m.PutStr("message", "dots")
		m.PutInt("a.b", 5)
		m.PutEmptyMap("a").PutInt("c", 1)
	})
	appendRecord(func(body pcommon.Value) {
		m := body.SetEmptyMap()
		m.PutStr("message", "promoted")
		m.PutEmptyMap("user").PutStr("id", "u1")
	})

	require.NoError(t, exp.pushLogsData(ctx, ld))

	var count uint64
	require.NoError(t, admin.QueryRow(ctx, `SELECT count() FROM signoz_logs.distributed_logs_v2`).Scan(&count))
	assert.Equal(t, uint64(6), count, "every pushed record must be stored")

	var bodyJSON, types string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_v2), toJSONString(JSONAllPathsWithTypes(body_v2)) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'habroker metrics'`,
	).Scan(&bodyJSON, &types))
	assert.Contains(t, bodyJSON, `"cgroup_memory_limit_bytes":18446744073709552000`)
	assert.JSONEq(t, `{"count":"Int64","flag":"Bool","message":"String","metrics.cgroup_memory_limit_bytes":"Float64","pi":"Float64"}`, types)

	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_v2), toJSONString(JSONAllPathsWithTypes(body_v2)) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'arrays'`,
	).Scan(&bodyJSON, &types))
	assert.JSONEq(t, `{"message":"arrays","tags":["x","y"],"nums":[1,null,2],"mixed":[1,"a",{"x":2},[3]],"objs":[{"k":1},{"k":2}]}`, bodyJSON)
	assert.JSONEq(t, `{"message":"String","tags":"Array(Nullable(String))","nums":"Array(Nullable(Int64))","mixed":"Array(Dynamic)","objs":"Array(JSON)"}`, types)

	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_v2) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'plain text line'`,
	).Scan(&bodyJSON))
	assert.JSONEq(t, `{"message":"plain text line"}`, bodyJSON)

	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_v2) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = '123'`,
	).Scan(&bodyJSON))
	assert.JSONEq(t, `{"message":"123"}`, bodyJSON)

	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_v2) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'dots'`,
	).Scan(&bodyJSON))
	assert.JSONEq(t, `{"message":"dots","a":{"b":5,"c":1}}`, bodyJSON)

	var promoted string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toJSONString(body_promoted) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'promoted'`,
	).Scan(&promoted))
	assert.JSONEq(t, `{"user":{"id":"u1"}}`, promoted)

	cfgOff := &Config{
		DSN:                       dsn,
		PromotedPathsSyncInterval: utils.ToPointer(5 * time.Minute),
		LogLevelConcurrency:       utils.ToPointer(1),
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: time.Minute,
			MaxDistinctValues: 25000,
		},
	}
	require.NoError(t, cfgOff.Validate())
	clientOff, err := newClickhouseClient(zap.NewNop(), cfgOff)
	require.NoError(t, err)
	optsOff := testOptions(t)
	optsOff = append(optsOff, WithClickHouseClient(clientOff), WithNewUsageCollector(uuid.New(), clientOff))
	expOff, err := newExporter(exporter.Settings{}, cfgOff, optsOff...)
	require.NoError(t, err)
	require.NoError(t, expOff.Start(ctx, nil))
	defer func() { require.NoError(t, expOff.Shutdown(ctx)) }()

	ldOff := plog.NewLogs()
	recOff := ldOff.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	recOff.SetTimestamp(now)
	recOff.SetObservedTimestamp(now)
	recOff.Body().SetStr("flag off line")
	require.NoError(t, expOff.pushLogsData(ctx, ldOff))

	var body string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT body, toJSONString(body_v2) FROM signoz_logs.distributed_logs_v2 WHERE body = 'flag off line'`,
	).Scan(&body, &bodyJSON))
	assert.Equal(t, "flag off line", body)
	assert.JSONEq(t, `{"message":""}`, bodyJSON)

	ldWide := plog.NewLogs()
	slWide := ldWide.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	for r := 0; r < 3; r++ {
		rec := slWide.LogRecords().AppendEmpty()
		rec.SetTimestamp(now)
		rec.SetObservedTimestamp(now)
		bodyMap := rec.Body().SetEmptyMap()
		bodyMap.PutStr("message", fmt.Sprintf("wide-%d", r))
		for k := 0; k < 500; k++ {
			bodyMap.PutInt(fmt.Sprintf("r%d_k%d", r, k), int64(k))
		}
	}
	require.NoError(t, exp.pushLogsData(ctx, ldWide))

	var wideCount uint64
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT count() FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message LIKE 'wide-%'`,
	).Scan(&wideCount))
	assert.Equal(t, uint64(3), wideCount, "chunked inserts must store every record")

	var wideVal string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT toString(body_v2.r2_k499) FROM signoz_logs.distributed_logs_v2 WHERE body_v2.message = 'wide-2'`,
	).Scan(&wideVal))
	assert.Equal(t, "499", wideVal)
}
