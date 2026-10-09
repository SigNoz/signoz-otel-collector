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
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
)

// The integration test needs a ClickHouse server with a cluster for the
// distributed tables. It recreates the signoz_metadata database.
//
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 go test -run Integration ./exporter/metadataexporter/internal/fieldvalues/
const (
	dsnEnv     = "FIELDVALUES_CLICKHOUSE_DSN"
	clusterEnv = "FIELDVALUES_CLICKHOUSE_CLUSTER"
)

func integrationConn(t *testing.T) driver.Conn {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skipf("set %s to run the ClickHouse integration test", dsnEnv)
	}
	cluster := os.Getenv(clusterEnv)
	if cluster == "" {
		cluster = "c1"
	}
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()
	require.NoError(t, conn.Exec(ctx, "DROP DATABASE IF EXISTS signoz_metadata SYNC"))
	require.NoError(t, conn.Exec(ctx, "CREATE DATABASE signoz_metadata"))
	var migration *schemamigrator.SchemaMigrationRecord
	for i := range schemamigrator.MetadataMigrations {
		if schemamigrator.MetadataMigrations[i].MigrationID == 1002 {
			migration = &schemamigrator.MetadataMigrations[i]
		}
	}
	require.NotNil(t, migration)
	for _, op := range migration.UpItems {
		if create, ok := op.(schemamigrator.CreateTableOperation); ok {
			if d, ok := create.Engine.(schemamigrator.Distributed); ok {
				d.Cluster = cluster
				create.Engine = d
			}
			op = create
		}
		require.NoError(t, conn.Exec(ctx, op.ToSQL()), op.ToSQL())
	}
	return conn
}

func flush(t *testing.T, conn driver.Conn) {
	t.Helper()
	require.NoError(t, conn.Exec(context.Background(), "SYSTEM FLUSH DISTRIBUTED signoz_metadata.distributed_field_values_sets"))
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
  AND last_seen >= toDateTime('2026-09-22 00:00:00', 'UTC')
  AND resource_hash IN (
      SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'logs' AND field_name = 'deployment.environment.name'
        AND field_context = 'resource' AND string_value = 'prod')
  AND %s IN (
      SELECT %s FROM signoz_metadata.distributed_field_values_sets AS c
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

	attrQuery := fmt.Sprintf(relatedQuery, "(resource_hash, attrs_hash)", "resource_hash, attrs_hash")
	assert.Equal(t, []string{"POST"}, queryStrings(t, conn, attrQuery, "http.method", "attribute", "2026-09-22 11:00:00"))
	assert.Equal(t, []string{"POST", "PUT"}, queryStrings(t, conn, attrQuery, "http.method", "attribute", "2026-09-22 15:00:00"))

	resourceQuery := fmt.Sprintf(relatedQuery, "resource_hash", "resource_hash")
	assert.Equal(t, []string{"checkout", "payments"}, queryStrings(t, conn, resourceQuery, "service.name", "resource", "2026-09-22 12:00:00"),
		"exclude-self: the service filter drops its own condition")

	plain := queryStrings(t, conn, `SELECT concat(string_value, ':', toString(uniqCombinedMerge(12)(holders)))
FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.method' AND field_context = 'attribute'
  AND field_data_type = 'string' AND day = '2026-09-22'
GROUP BY string_value`)
	assert.Equal(t, []string{"GET:2", "POST:1", "PUT:1"}, plain, "plain values with holders from the view")

	require.NoError(t, e.WriteLogs(context.Background(), exampleLogs()))
	flush(t, conn)
	var rows uint64
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT count() FROM signoz_metadata.field_values_sets").Scan(&rows))
	assert.Equal(t, uint64(14), rows, "the second push of the same day writes nothing")
}

func TestIntegrationClassifierFindsFieldsOverTheLimit(t *testing.T) {
	conn := integrationConn(t)
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	now := time.Now().UTC().Truncate(time.Minute)
	setNow(e, now)

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

	values := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND metric_name = '' AND field_name = 'user.id' AND day = toDate(now(), 'UTC')
GROUP BY string_value`)
	assert.Equal(t, []string{"u0", "u1", "u2", "u3"}, values, "the collector writes the limit plus one values")

	e.refresh()
	class := e.class.Load()
	require.NotNil(t, class)
	assert.True(t, class.isOver(fieldIDOf(contextAttribute, "user.id")))
	assert.False(t, class.isOver(fieldIDOf(contextAttribute, "http.method")))

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
	flush(t, conn)

	keys := queryStrings(t, conn, `SELECT concat(field_context, ':', field_name)
FROM signoz_metadata.distributed_field_keys_daily
WHERE signal = 'metrics' AND source = 'meter' AND metric_name = 'http_requests'
GROUP BY field_context, field_name`)
	assert.Equal(t, []string{"attribute:code", "attribute:method", "resource:k8s.pod.name", "resource:service.name", "scope:library.lang"}, keys,
		"the key view has the keys of a metric, its resource keys from the key rows")
	assert.Empty(t, queryStrings(t, conn, `SELECT field_name FROM signoz_metadata.distributed_field_keys_daily
WHERE signal = 'metrics' AND field_name = '__name__' GROUP BY field_name`), "link rows are not keys")

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

	assert.Empty(t, queryStrings(t, conn, `SELECT field_name FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'metrics' AND metric_name != '' AND field_context = 'resource' GROUP BY field_name`),
		"key rows stay out of the daily view")

	assert.Empty(t, queryStrings(t, conn, `SELECT field_name FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'metrics' AND source = '' GROUP BY field_name`), "the meter source is its own space")

	allMetrics := queryStrings(t, conn, `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'metrics' AND source = 'meter' AND metric_name = '' AND field_name = 'code' GROUP BY string_value`)
	assert.Equal(t, []string{"200", "500"}, allMetrics, "the view also writes each metrics row under the empty metric name")
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
	require.NoError(t, conn.QueryRow(context.Background(), "SELECT count() FROM signoz_metadata.field_values_sets").Scan(&rows))
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
		"user": map[string]any{"plan": "pro", "seats": 5},
	}))
	require.NoError(t, e.WriteLogs(context.Background(), ld))
	flush(t, conn)
	assert.Equal(t, []string{"user.plan:string:pro", "user.seats:number:5"}, queryStrings(t, conn, `SELECT concat(field_name, ':', toString(field_data_type), ':', if(field_data_type = 'number', toString(number_value), string_value))
FROM signoz_metadata.distributed_field_values_daily WHERE signal = 'logs' AND field_context = 'body' GROUP BY field_name, field_data_type, string_value, number_value`))
}

// Rows written ahead and late records land on the day of their window in the
// pair table and in the view.
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
		"GET 2026-09-22 10:00:00 2026-09-23 00:00:00",
		"PUT 2026-09-23 00:00:00 2026-09-23 00:00:00",
	}, queryStrings(t, conn, `SELECT concat(string_value, ' ', toString(min(first_seen), 'UTC'), ' ', toString(max(last_seen), 'UTC'))
FROM signoz_metadata.field_values_sets WHERE field_name = 'http.method' GROUP BY string_value ORDER BY string_value`))
	assert.Equal(t, []string{"GET 2026-09-22", "GET 2026-09-23", "PUT 2026-09-23"},
		queryStrings(t, conn, `SELECT concat(string_value, ' ', toString(day)) FROM signoz_metadata.field_values_daily
WHERE field_name = 'http.method' GROUP BY string_value, day ORDER BY string_value, day`))
}
