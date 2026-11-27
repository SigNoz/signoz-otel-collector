package fieldvalues

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
)

// The integration test needs a ClickHouse server with a cluster for the
// distributed tables. It recreates the signoz_metadata database on every node
// of the cluster. FIELDVALUES_CLICKHOUSE_REPLICATED=1 makes replicated tables,
// for a cluster with replicas and a keeper.
//
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 go test -p 1 -run Integration ./exporter/metadataexporter/internal/fieldvalues/
const (
	dsnEnv        = "FIELDVALUES_CLICKHOUSE_DSN"
	clusterEnv    = "FIELDVALUES_CLICKHOUSE_CLUSTER"
	replicatedEnv = "FIELDVALUES_CLICKHOUSE_REPLICATED"
)

func integrationCluster() string {
	if cluster := os.Getenv(clusterEnv); cluster != "" {
		return cluster
	}
	return "c1"
}

func integrationConn(t *testing.T) driver.Conn {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skipf("set %s to run the ClickHouse integration test", dsnEnv)
	}
	cluster := integrationCluster()
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()
	require.NoError(t, conn.Exec(ctx, "DROP DATABASE IF EXISTS signoz_metadata ON CLUSTER "+cluster+" SYNC"))
	require.NoError(t, conn.Exec(ctx, "CREATE DATABASE signoz_metadata ON CLUSTER "+cluster))
	var migration *schemamigrator.SchemaMigrationRecord
	for i := range schemamigrator.MetadataMigrations {
		if schemamigrator.MetadataMigrations[i].MigrationID == 1003 {
			migration = &schemamigrator.MetadataMigrations[i]
		}
	}
	require.NotNil(t, migration)
	for _, op := range migration.UpItems {
		op = op.OnCluster(cluster)
		if os.Getenv(replicatedEnv) != "" {
			op = op.WithReplication()
		}
		require.NoError(t, conn.Exec(ctx, op.ToSQL()), op.ToSQL())
	}
	return conn
}

// flush sends the async inserts on to the shards, and the shards' inserts on
// to the tables. The inserts that the distributed table sends to a shard keep
// the async settings, so the shard queues them again.
func flush(t *testing.T, conn driver.Conn) {
	t.Helper()
	cluster := integrationCluster()
	queries := []string{
		"SYSTEM FLUSH ASYNC INSERT QUEUE ON CLUSTER " + cluster,
		"SYSTEM FLUSH DISTRIBUTED ON CLUSTER " + cluster + " signoz_metadata.distributed_field_values_sets",
		"SYSTEM FLUSH ASYNC INSERT QUEUE ON CLUSTER " + cluster,
	}
	if os.Getenv(replicatedEnv) != "" {
		queries = append(queries, "SYSTEM SYNC REPLICA ON CLUSTER "+cluster+" signoz_metadata.field_values_sets")
	}
	for _, q := range queries {
		require.NoError(t, conn.Exec(context.Background(), q), q)
	}
}

func queryStrings(t *testing.T, conn driver.Conn, query string, args ...any) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), query, args...)
	require.NoError(t, err, query)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		out = append(out, v)
	}
	require.NoError(t, rows.Err())
	sort.Strings(out)
	return out
}

func newIntegrationExporter(t *testing.T, conn driver.Conn, cfg Config, signal pipeline.Signal) *Writer {
	t.Helper()
	e, err := New(cfg, Settings{Signal: signal, Conn: conn, Logger: zap.NewNop(), Telemetry: componenttest.NewNopTelemetrySettings()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Shutdown() })
	return e
}

func exampleLogs() plog.Logs {
	ld := plog.NewLogs()
	add := func(service, env string, records ...[3]string) {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", service)
		rl.Resource().Attributes().PutStr("deployment.environment.name", env)
		sl := rl.ScopeLogs().AppendEmpty()
		for _, r := range records {
			lr := sl.LogRecords().AppendEmpty()
			lr.SetTimestamp(at(r[0]))
			lr.SetSeverityText(r[2])
			lr.Attributes().PutStr("http.method", r[1])
		}
	}
	add("checkout", "prod", [3]string{"10:05", "GET", "INFO"}, [3]string{"10:20", "POST", "ERROR"})
	add("checkout", "staging", [3]string{"10:30", "GET", "ERROR"})
	add("payments", "prod", [3]string{"11:10", "PUT", "ERROR"})
	return ld
}

// relatedQuery is the read of Example 1 of the proposal, on the distributed
// tables. The subqueries are answered by each shard from its own rows, which
// is correct because the pair table is sharded by resource. The old analyzer
// needs an alias for each distributed table in this mode.
const relatedQuery = `SELECT string_value
FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = ? AND field_context = ?
  AND first_seen < toDateTime(?, 'UTC')
  AND last_seen >= toDateTime('%[1]s 00:00:00', 'UTC')
  AND resource_hash IN (
      SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'logs' AND field_name = 'deployment.environment.name'
        AND field_context = 'resource' AND string_value = 'prod')
  AND %[2]s IN (
      SELECT %[3]s FROM signoz_metadata.distributed_field_values_sets AS c
      WHERE signal = 'logs' AND field_name = 'severity_text'
        AND field_context = 'log' AND string_value = 'ERROR')
GROUP BY string_value
ORDER BY uniq(resource_hash, attrs_hash) DESC
LIMIT 51
SETTINGS distributed_product_mode = 'local'`

func TestIntegrationExampleOneQuickFilter(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(e, testDay)
	require.NoError(t, e.WriteLogs(context.Background(), exampleLogs()))
	flush(t, conn)

	attrQuery := fmt.Sprintf(relatedQuery, testDate, "(resource_hash, attrs_hash)", "resource_hash, attrs_hash")
	assert.Equal(t, []string{"POST"}, queryStrings(t, conn, attrQuery, "http.method", "attribute", testDate+" 11:00:00"))
	assert.Equal(t, []string{"POST", "PUT"}, queryStrings(t, conn, attrQuery, "http.method", "attribute", testDate+" 15:00:00"))

	resourceQuery := fmt.Sprintf(relatedQuery, testDate, "resource_hash", "resource_hash")
	assert.Equal(t, []string{"checkout", "payments"}, queryStrings(t, conn, resourceQuery, "service.name", "resource", testDate+" 12:00:00"),
		"exclude-self: the service filter drops its own condition")

	plain := queryStrings(t, conn, `SELECT concat(string_value, ':', toString(count()))
FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.method' AND field_context = 'attribute'
  AND toDate(last_seen, 'UTC') >= ?
GROUP BY string_value, number_value`, testDate)
	assert.Equal(t, []string{"GET:2", "POST:1", "PUT:1"}, plain, "plain values with the count of their sets")

	require.NoError(t, e.WriteLogs(context.Background(), exampleLogs()))
	flush(t, conn)
	var rows uint64
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT count() FROM signoz_metadata.distributed_field_values_sets").Scan(&rows))
	assert.Equal(t, uint64(14), rows, "the second push of the same day writes nothing")
}

func TestIntegrationClassifierFindsFieldsOverTheLimit(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	// Noon of today, so the classifier reads today's rows, and the time does
	// not fall in the spread of the daily sample.
	setNow(e, time.Now().UTC().Truncate(24*time.Hour).Add(12*time.Hour))

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	sl := rl.ScopeLogs().AppendEmpty()
	for i := 0; i < 6; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(0)
		lr.SetObservedTimestamp(0)
		lr.Attributes().PutStr("http.method", "GET")
		lr.Attributes().PutStr("user.id", fmt.Sprintf("u%d", i))
	}
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	flush(t, conn)

	values := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'user.id' AND field_context = 'attribute'
  AND toDate(last_seen, 'UTC') >= toDate(now(), 'UTC')
GROUP BY string_value, number_value`)
	assert.Equal(t, []string{"u0", "u1", "u2", "u3"}, values, "the collector writes the limit plus one values")

	userID, method := fieldIDOf(contextAttribute, "user.id"), fieldIDOf(contextAttribute, "http.method")
	overRows := func() []string {
		require.NoError(t, conn.Exec(context.Background(), "SYSTEM FLUSH DISTRIBUTED ON CLUSTER "+integrationCluster()+" signoz_metadata.distributed_field_days_over_limit"))
		return queryStrings(t, conn, `SELECT concat(signal, ' ', toString(field_context), ' ', field_name, ' ', toString(day = toDate(now(), 'UTC')))
FROM signoz_metadata.distributed_field_days_over_limit`)
	}

	// A second collector has no count of its own. It finds the field in the
	// rows of the first one, and writes the row of the field and the day.
	other := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(other, time.Now().UTC().Truncate(24*time.Hour).Add(12*time.Hour))
	other.refresh(refreshTimeout)
	require.NotNil(t, other.class.Load())
	assert.True(t, other.class.Load().isOver(userID))
	assert.False(t, other.class.Load().isOver(method))
	assert.Equal(t, []string{"logs attribute user.id 1"}, overRows())

	// The first collector has the field in its own count. The row is there,
	// so it writes no second one.
	e.refresh(refreshTimeout)
	assert.True(t, e.class.Load().isOver(userID))
	assert.Equal(t, []string{"logs attribute user.id 1"}, overRows())

	inHash := queryStrings(t, conn, `SELECT concat(field_name, ':', toString(min(in_hash)))
FROM signoz_metadata.distributed_field_values_sets WHERE field_name IN ('http.method', 'user.id') GROUP BY field_name, string_value`)
	assert.Equal(t, "http.method:true,user.id:false,user.id:true,user.id:true,user.id:true", strings.Join(inHash, ","))
}

func TestIntegrationMetricKeysAndSource(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Source = "meter"
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalMetrics)
	setNow(e, testDay)
	require.NoError(t, e.WriteMetrics(context.Background(), testMetrics()))
	e.flushKeys()
	flush(t, conn)
	if os.Getenv(replicatedEnv) != "" {
		require.NoError(t, conn.Exec(context.Background(), "SYSTEM SYNC REPLICA ON CLUSTER "+integrationCluster()+" signoz_metadata.field_keys_v2"))
	}

	keys := queryStrings(t, conn, `SELECT concat(field_context, ':', field_name)
FROM signoz_metadata.distributed_field_keys_v2
WHERE signal = 'metrics' AND source = 'meter' AND metric_name = 'http_requests'
GROUP BY field_context, field_name`)
	assert.Equal(t, []string{"attribute:code", "attribute:method", "resource:k8s.pod.name", "resource:service.name", "scope:library.lang"}, keys,
		"the keys of a metric")

	podValues := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'metrics' AND source = 'meter' AND metric_name = '' AND field_name = 'k8s.pod.name' AND field_context = 'resource'
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS l
      WHERE signal = 'metrics' AND source = 'meter' AND metric_name = '' AND field_name = '__name__' AND string_value = 'http_requests')
GROUP BY string_value
SETTINGS distributed_product_mode = 'local'`)
	assert.Equal(t, []string{"p1"}, podValues, "the values of a resource key for a metric, through its link rows")

	metrics := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS l
WHERE signal = 'metrics' AND source = 'meter' AND metric_name = '' AND field_name = '__name__'
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'metrics' AND source = 'meter' AND metric_name = '' AND field_name = 'service.name' AND string_value = 'checkout')
GROUP BY string_value
SETTINGS distributed_product_mode = 'local'`)
	assert.Equal(t, []string{"http_requests", "latency.count"}, metrics, "the metrics of a resource")

	assert.Empty(t, queryStrings(t, conn, `SELECT field_name FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'metrics' AND source = '' GROUP BY field_name`), "the meter source is its own space")

	allMetrics := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'metrics' AND source = 'meter' AND field_name = 'code' AND field_context = 'attribute'
  AND toDate(last_seen, 'UTC') >= ?
GROUP BY string_value, number_value`, testDate)
	assert.Equal(t, []string{"200", "500"}, allMetrics, "the values of a label for all metrics need no metric name")
}

// A resource field over the limit leaves the identity of the metric resources,
// and the reads through the link rows still give all of its values.
func TestIntegrationMetricResourceFieldOverTheLimit(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Limits.MaxResourceFieldValues = 10
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalMetrics)
	setNow(e, testDay)
	require.NoError(t, e.WriteMetrics(context.Background(), processMetrics(200)))
	flush(t, conn)

	pids := queryStrings(t, conn, `SELECT toString(uniqExact(string_value)) FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'metrics' AND metric_name = '' AND field_name = 'process.pid' AND field_context = 'resource'
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS l
      WHERE signal = 'metrics' AND metric_name = '' AND field_name = '__name__' AND string_value = 'process.cpu.time')
SETTINGS distributed_product_mode = 'local'`)
	assert.Equal(t, []string{"200"}, pids, "the pids of a metric, through its link rows")

	resources := queryStrings(t, conn, `SELECT toString(uniqExact(resource_hash)) FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'metrics' AND metric_name = 'process.cpu.time'`)
	assert.Equal(t, []string{"11"}, resources, "10 processes in full, then one resource for the host")
}

func TestIntegrationSharedCacheWritesEachRowOnce(t *testing.T) {
	conn := integrationConn(t)
	shared := newMapCache()
	writers := make([]*Writer, 3)
	for i := range writers {
		w, err := New(testConfig(), Settings{Signal: pipeline.SignalLogs, Conn: conn, Logger: zap.NewNop(), Telemetry: componenttest.NewNopTelemetrySettings(), Shared: shared})
		require.NoError(t, err)
		setNow(w, testDay)
		writers[i] = w
		t.Cleanup(func() { _ = w.Shutdown() })
	}
	for _, w := range writers {
		require.NoError(t, w.WriteLogs(context.Background(), exampleLogs()))
	}
	flush(t, conn)
	var rows uint64
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT count() FROM signoz_metadata.distributed_field_values_sets").Scan(&rows))
	assert.Equal(t, uint64(14), rows, "three collectors with a shared cache insert the rows of one")
}

func TestIntegrationBodyPairs(t *testing.T) {
	conn := integrationConn(t)
	e, err := New(testConfig(), Settings{Signal: pipeline.SignalLogs, Conn: conn, Logger: zap.NewNop(), Telemetry: componenttest.NewNopTelemetrySettings(), BodyJSON: &testBodyLimits})
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Shutdown() })
	setNow(e, testDay)
	ld := logsOf(checkout, logRecord{"10:00", map[string]any{"http.method": "GET"}})
	require.NoError(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetEmptyMap().FromRaw(map[string]any{
		"user":    map[string]any{"plan": "pro", "seats": 5},
		"message": "order placed",
		"items":   []any{map[string]any{"sku": "a1"}},
	}))
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	e.flushKeys()
	assert.Equal(t, []string{"items:[]json", "items[].sku:string", "message:string", "user.plan:string", "user.seats:int64"}, queryStrings(t, conn, `SELECT concat(field_name, ':', field_data_type)
FROM signoz_metadata.distributed_field_keys_v2 WHERE signal = 'logs' AND field_context = 'body' GROUP BY field_name, field_data_type ORDER BY field_name`))
	flush(t, conn)
	assert.Equal(t, []string{"items[].sku:string:a1", "user.plan:string:pro", "user.seats:number:5"}, queryStrings(t, conn, `SELECT concat(field_name, ':', toString(field_data_type), ':', if(field_data_type = 'number', toString(number_value), string_value))
FROM signoz_metadata.distributed_field_values_sets WHERE signal = 'logs' AND field_context = 'body' GROUP BY field_name, field_data_type, string_value, number_value`))
}

// A set seen at the end of a day gets no row of the next day. A late record
// lands on the day of its window.
func TestIntegrationRowsLandOnTheDayOfTheirWindow(t *testing.T) {
	conn := integrationConn(t)
	e := newIntegrationExporter(t, conn, testConfig(), pipeline.SignalLogs)
	ctx := context.Background()
	setNow(e, testDay)
	require.NoError(t, e.WriteLogs(ctx, logsOf(checkout, logRecord{"10:00", map[string]any{"http.method": "GET"}})))
	setNow(e, lastMillisecond)
	require.NoError(t, e.WriteLogs(ctx, logsAt(lastMillisecond, checkout, map[string]any{"http.method": "GET"})))
	setNow(e, midnight.Add(5*time.Minute))
	require.NoError(t, e.WriteLogs(ctx, logsAt(midnight.Add(-time.Minute), checkout, map[string]any{"http.method": "PUT"})))
	flush(t, conn)

	assert.Equal(t, []string{
		"GET " + testDate + " 10:00:00 " + testDate + " 10:00:00",
		"PUT " + testNextDate + " 00:00:00 " + testNextDate + " 00:00:00",
	}, queryStrings(t, conn, `SELECT concat(string_value, ' ', toString(min(first_seen), 'UTC'), ' ', toString(max(last_seen), 'UTC'))
FROM signoz_metadata.distributed_field_values_sets WHERE field_name = 'http.method' GROUP BY string_value ORDER BY string_value`))
}

func TestIntegrationInsertsAreAsync(t *testing.T) {
	conn := integrationConn(t)
	e := newIntegrationExporter(t, conn, testConfig(), pipeline.SignalLogs)
	setNow(e, testDay)
	// The clock of the server gives the start, as query_log has its times.
	var since int64
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT toUnixTimestamp64Micro(now64(6))").Scan(&since))
	require.NoError(t, e.WriteLogs(context.Background(), exampleLogs()))
	flush(t, conn)
	require.NoError(t, conn.Exec(context.Background(), "SYSTEM FLUSH LOGS"))

	// query_log lists only the settings that differ from the defaults of the
	// server.
	inserts := queryStrings(t, conn, `SELECT concat(
    if(Settings['async_insert'] = '', (SELECT value FROM system.settings WHERE name = 'async_insert'), Settings['async_insert']), ' ',
    Settings['wait_for_async_insert'], ' ', Settings['async_insert_busy_timeout_max_ms'])
FROM system.query_log
WHERE type = 'QueryFinish' AND query_kind = 'Insert' AND is_initial_query AND event_time_microseconds >= fromUnixTimestamp64Micro(?)
  AND has(tables, 'signoz_metadata.distributed_field_values_sets')`, since)
	assert.Equal(t, []string{"1 0 1000"}, inserts, "one insert for one push, async, with no wait and a busy timeout of 1 s")
	var rows uint64
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT count() FROM signoz_metadata.distributed_field_values_sets").Scan(&rows))
	assert.Equal(t, uint64(14), rows)
}

// ClickHouse reads the projection only for a statement of its shape, and it
// reads the rows of the sets with no notice for any other. EXPLAIN names the
// projection when the statement can use it, whatever the size of the table.
func TestIntegrationReadsCanUseTheProjection(t *testing.T) {
	conn := integrationConn(t)
	// EXPLAIN plans no read of an empty table. The rows go to the table of
	// this node, where EXPLAIN runs.
	require.NoError(t, conn.Exec(context.Background(), `INSERT INTO signoz_metadata.field_values_sets
(signal, source, metric_name, field_context, field_name, field_data_type, string_value, resource_hash, attrs_hash, in_hash, first_seen, last_seen, inserted_at)
VALUES ('logs', '', '', 'attribute', 'http.method', 'string', 'GET', 1, 1, true, now(), now(), now()),
       ('metrics', '', 'http_requests', 'attribute', 'code', 'string', '200', 1, 1, true, now(), now(), now())`))
	skip := []clickhouse.GroupSet{{Value: []any{"", "attribute", "user.id"}}}
	for _, tc := range []struct {
		name   string
		query  string
		args   []any
		usable bool
	}{
		{"fields over the limit", fmt.Sprintf(overLimitQuery, ""), []any{"logs", "", testDate, 100000, 5000}, true},
		{"fields over the limit, without the flagged fields", fmt.Sprintf(overLimitQuery, notFlagged), []any{"logs", "", testDate, skip, 100000, 5000}, true},
		{"known fields", fmt.Sprintf(knownFieldsQuery, ""), []any{"logs", "", testDate, 100}, true},
		{"known fields, without the flagged fields", fmt.Sprintf(knownFieldsQuery, notFlagged), []any{"logs", "", testDate, skip, 100}, true},
		{"values of a field", `SELECT string_value, number_value FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.method' AND field_context = 'attribute'
  AND toDate(last_seen, 'UTC') >= ?
GROUP BY string_value, number_value LIMIT 2001`, []any{testDate}, true},
		{"values of a label of all metrics", `SELECT string_value, number_value FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'metrics' AND source = '' AND field_name = 'code' AND field_context = 'attribute'
  AND toDate(last_seen, 'UTC') >= ?
GROUP BY string_value, number_value LIMIT 2001`, []any{testDate}, true},
		{"a condition on last_seen itself", `SELECT string_value FROM signoz_metadata.distributed_field_values_sets
WHERE signal = 'logs' AND field_name = 'http.method' AND last_seen >= toDateTime(?, 'UTC')
GROUP BY string_value`, []any{testDate + " 00:00:00"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := strings.ReplaceAll(tc.query, "distributed_field_values_sets", "field_values_sets")
			plan := strings.Join(queryStrings(t, conn, "EXPLAIN projections = 1 "+local, tc.args...), "\n")
			assert.Equal(t, tc.usable, strings.Contains(plan, "p_field_values"), plan)
		})
	}
}

func userLogs(users int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	sl := rl.ScopeLogs().AppendEmpty()
	for i := 0; i < users; i++ {
		sl.LogRecords().AppendEmpty().Attributes().PutStr("user.id", fmt.Sprintf("u%d", i))
	}
	return ld
}

func overLimitRows(t *testing.T, conn driver.Conn) []string {
	t.Helper()
	require.NoError(t, conn.Exec(context.Background(), "SYSTEM FLUSH DISTRIBUTED ON CLUSTER "+integrationCluster()+" signoz_metadata.distributed_field_days_over_limit"))
	return queryStrings(t, conn, `SELECT concat(signal, ' ', if(metric_name = '', '', concat(metric_name, ' ')), toString(field_context), ' ', field_name, ' ', toString(day))
FROM signoz_metadata.distributed_field_days_over_limit GROUP BY signal, metric_name, field_context, field_name, day`)
}

// A field that passes its limit after the last refresh of a day gets its row
// from the read for the day that ended. That read counts all values since
// the start of that day, so it is correct only right after the day change.
func TestIntegrationClassifierReadsTheDayThatEnded(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	today := time.Now().UTC().Truncate(24 * time.Hour)
	userID := fieldIDOf(contextAttribute, "user.id")

	yesterday := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(yesterday, today.Add(-12*time.Hour))
	require.NoError(t, yesterday.WriteLogs(context.Background(), userLogs(6)))
	flush(t, conn)

	late := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(late, today.Add(cfg.Classification.RefreshInterval+5*time.Minute))
	late.refresh(refreshTimeout)
	require.NotNil(t, late.class.Load())
	assert.False(t, late.class.Load().isOver(userID), "after the first interval of the day, the day that ended is not read")
	assert.Empty(t, overLimitRows(t, conn))

	inTime := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(inTime, today.Add(5*time.Minute))
	inTime.refresh(refreshTimeout)
	require.NotNil(t, inTime.class.Load())
	assert.True(t, inTime.class.Load().isOver(userID))
	assert.Equal(t, []string{"logs attribute user.id " + today.AddDate(0, 0, -1).Format(time.DateOnly)}, overLimitRows(t, conn))
}

func TestIntegrationClassifierLookback(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	today := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(t, conn.Exec(context.Background(), `INSERT INTO signoz_metadata.distributed_field_days_over_limit
(signal, source, field_context, field_name, day) VALUES ('logs', '', 'attribute', 'last.day', ?), ('logs', '', 'attribute', 'day.before', ?), ('traces', '', 'attribute', 'other.signal', ?)`,
		today.AddDate(0, 0, -(cfg.Classification.LookbackDays-1)).Format(time.DateOnly),
		today.AddDate(0, 0, -cfg.Classification.LookbackDays).Format(time.DateOnly),
		today.Format(time.DateOnly)))
	overLimitRows(t, conn)

	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	setNow(e, today.Add(12*time.Hour))
	e.refresh(refreshTimeout)
	class := e.class.Load()
	require.NotNil(t, class)
	assert.True(t, class.isOver(fieldIDOf(contextAttribute, "last.day")), "the last day of the lookback")
	assert.False(t, class.isOver(fieldIDOf(contextAttribute, "day.before")))
	assert.False(t, class.isOver(fieldIDOf(contextAttribute, "other.signal")))
}

// The limit of a metric label is for each metric.
func TestIntegrationClassifierFindsMetricLabelsOverTheLimit(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	noon := time.Now().UTC().Truncate(24 * time.Hour).Add(12 * time.Hour)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	sm := rm.ScopeMetrics().AppendEmpty()
	for name, pids := range map[string]int{"http_requests": 4, "http_errors": 2} {
		m := sm.Metrics().AppendEmpty()
		m.SetName(name)
		gauge := m.SetEmptyGauge()
		for pid := 0; pid < pids; pid++ {
			dp := gauge.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("pid", fmt.Sprintf("%d", pid))
			dp.Attributes().PutStr("method", "GET")
		}
	}
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalMetrics)
	setNow(e, noon)
	require.NoError(t, e.WriteMetrics(context.Background(), md))
	flush(t, conn)

	other := newIntegrationExporter(t, conn, cfg, pipeline.SignalMetrics)
	setNow(other, noon)
	other.refresh(refreshTimeout)
	class := other.class.Load()
	require.NotNil(t, class)
	pid := fieldIDOf(contextAttribute, "pid")
	assert.True(t, class.isOver(limitField(nameHash("http_requests"), pid)))
	assert.False(t, class.isOver(limitField(nameHash("http_errors"), pid)), "the same label of a metric with few values")
	assert.False(t, class.isOver(limitField(nameHash("http_requests"), fieldIDOf(contextAttribute, "method"))))
	assert.Equal(t, []string{"metrics http_requests attribute pid " + noon.Format(time.DateOnly)}, overLimitRows(t, conn))
}

// The keys are in the table when the flush returns, with no flush of a queue
// by the test: the writer marks a key as written only then. A key has one row
// however many days wrote it.
func TestIntegrationKeysAreInTheTableWhenTheFlushReturns(t *testing.T) {
	conn := integrationConn(t)
	ctx := context.Background()
	logs := newIntegrationExporter(t, conn, testConfig(), pipeline.SignalLogs)
	meterConfig := testConfig()
	meterConfig.Source = "meter"
	meter := newIntegrationExporter(t, conn, meterConfig, pipeline.SignalMetrics)
	read := func() []string {
		if os.Getenv(replicatedEnv) != "" {
			require.NoError(t, conn.Exec(ctx, "SYSTEM SYNC REPLICA ON CLUSTER "+integrationCluster()+" signoz_metadata.field_keys_v2"))
		}
		return queryStrings(t, conn, `SELECT concat(signal, ' ', source, ' ', metric_name, ' ', field_context, ' ', field_name, ' ', field_data_type, ' ', toString(toDate(max(last_seen), 'UTC')), ' ', toString(count()))
FROM signoz_metadata.distributed_field_keys_v2
GROUP BY signal, source, metric_name, field_context, field_name, field_data_type`)
	}
	want := func(date, rows string) []string {
		keys := []string{
			"logs   attribute http.method string",
			"logs   attribute http.status int64",
			"logs   resource service.name string",
			"metrics meter  resource k8s.pod.name string",
			"metrics meter  resource service.name string",
			"metrics meter http_requests attribute code string",
			"metrics meter http_requests attribute method string",
			"metrics meter http_requests resource k8s.pod.name string",
			"metrics meter http_requests resource service.name string",
			"metrics meter http_requests scope library.lang string",
			"metrics meter latency.count attribute route string",
			"metrics meter latency.count resource k8s.pod.name string",
			"metrics meter latency.count resource service.name string",
			"metrics meter latency.count scope library.lang string",
		}
		for i := range keys {
			keys[i] += " " + date + " " + rows
		}
		return keys
	}
	push := func(at time.Time) {
		setNow(logs, at)
		setNow(meter, at)
		require.NoError(t, logs.WriteLogs(ctx, logsAt(at, map[string]any{"service.name": "checkout"}, map[string]any{"http.method": "GET", "http.status": int64(200)})))
		md := testMetrics()
		setTimestamps(md, at)
		require.NoError(t, meter.WriteMetrics(ctx, md))
		logs.flushKeys()
		meter.flushKeys()
	}

	push(testDay)
	assert.Equal(t, want(testDate, "1"), read())

	push(testDay.Add(time.Hour))
	assert.Equal(t, want(testDate, "1"), read(), "a key is written once a day")

	push(testDay.Add(24 * time.Hour))
	require.NoError(t, conn.Exec(ctx, "OPTIMIZE TABLE signoz_metadata.field_keys_v2 ON CLUSTER "+integrationCluster()+" FINAL"))
	assert.Equal(t, want(testNextDate, "1"), read(), "the row of the next day replaces the row of the day before")
}
