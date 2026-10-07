package metadataexporter

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvaluestest"
	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
)

// TestPerfFieldValues is a load test against a local ClickHouse. It pushes the
// same generated logs, spans and metrics through the metadata exporter with
// field_values off and on, and reports the cost of each, then times the reads
// of the new store.
//
//	FIELDVALUES_PERF=1 FIELDVALUES_PERF_BATCHES=300 \
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 \
//	go test -run TestPerfFieldValues -v -timeout 30m ./exporter/metadataexporter/
func TestPerfFieldValues(t *testing.T) {
	if os.Getenv("FIELDVALUES_PERF") == "" {
		t.Skip("set FIELDVALUES_PERF=1 to run the load test")
	}
	conn, dsn := integrationSchema(t)
	batches := 300
	if v := os.Getenv("FIELDVALUES_PERF_BATCHES"); v != "" {
		_, err := fmt.Sscan(v, &batches)
		require.NoError(t, err)
	}
	const batchSize = 1000
	repeats := 3
	if v := os.Getenv("FIELDVALUES_PERF_REPEATS"); v != "" {
		_, err := fmt.Sscan(v, &repeats)
		require.NoError(t, err)
	}

	var report strings.Builder
	fmt.Fprintf(&report, "batches per signal: %d of %d items, correlated data, 200 resources\n\n", batches, batchSize)
	fmt.Fprintf(&report, "Median of %d runs per mode, in alternating order. CPU is the collector process with the garbage collector paused (allocations are listed instead). Insert time is the ClickHouse duration of the inserts of each store, views included; macOS gives ClickHouse no per-query CPU.\n\n", repeats)
	fmt.Fprintf(&report, "| signal | field_values | push time | CPU | CPU / item | allocated / item | heap after | rows: attributes_metadata | rows: field_values_sets | inserts: attributes_metadata | inserts: field_values_sets | insert time: attributes_metadata | insert time: field values |\n")
	fmt.Fprintf(&report, "|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")

	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		results := map[bool][]perfResult{}
		for r := 0; r < repeats; r++ {
			// Alternate the order, so warm-up does not favour one mode.
			for _, on := range []bool{r%2 == 1, r%2 == 0} {
				truncate(t, conn)
				results[on] = append(results[on], runPerf(t, conn, dsn, signal, on, batches, batchSize))
			}
		}
		for _, on := range []bool{false, true} {
			res := median(results[on])
			fmt.Fprintf(&report, "| %s | %v | %s | %s | %s | %s | %s | %d | %d | %d | %d | %s | %s |\n",
				signal, on, res.wall.Round(time.Millisecond), res.cpu.Round(time.Millisecond),
				time.Duration(int64(res.cpu)/int64(batches*batchSize)), bytesString(res.allocated/uint64(batches*batchSize)),
				bytesString(res.heapAfter), res.attributesRows, res.fieldValuesRows, res.attributesInserts, res.fieldValuesInserts,
				res.attributesServerCPU.Round(time.Millisecond), res.fieldValuesServerCPU.Round(time.Millisecond))
		}
	}

	// Reads on the data of the last run of each signal, written again with
	// field_values on.
	truncate(t, conn)
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		runPerf(t, conn, dsn, signal, true, batches, batchSize)
	}
	fmt.Fprintf(&report, "\n%s\n", readReport(t, conn))
	fmt.Fprintf(&report, "\n%s\n", sizeReport(t, conn))

	t.Log("\n" + report.String())
	if path := os.Getenv("FIELDVALUES_PERF_REPORT"); path != "" {
		require.NoError(t, os.WriteFile(path, []byte(report.String()), 0o644))
	}
}

type perfResult struct {
	wall, cpu          time.Duration
	allocated          uint64
	heapAfter          uint64
	attributesRows     uint64
	fieldValuesRows    uint64
	fieldValuesInserts uint64
	attributesInserts  uint64

	attributesServerCPU  time.Duration
	fieldValuesServerCPU time.Duration
}

// median takes the run with the median collector CPU.
func median(runs []perfResult) perfResult {
	sorted := append([]perfResult(nil), runs...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].cpu < sorted[b].cpu })
	return sorted[len(sorted)/2]
}

func runPerf(t *testing.T, conn driver.Conn, dsn string, signal pipeline.Signal, on bool, batches, batchSize int) perfResult {
	t.Helper()
	ctx := context.Background()
	cfg := createDefaultConfig().(*Config)
	cfg.DSN = dsn
	cfg.Enabled = true
	cfg.FieldValues.Enabled = on
	cfg.FieldValues.Cache.MaxBytes = 64 << 20
	set := exportertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = componenttest.NewNopTelemetrySettings()
	e, err := newMetadataExporter(ctx, *cfg, set, signal)
	require.NoError(t, err)

	gen := fieldvaluestest.NewGenerator(42, time.Now())
	gen.Correlated = true
	since := time.Now()
	var res perfResult
	// The batches are made before each timed chunk, so the cost of the
	// generator is not measured.
	const chunk = 25
	for done := 0; done < batches; done += chunk {
		var pushes []func() error
		for i := done; i < min(done+chunk, batches); i++ {
			switch signal {
			case pipeline.SignalLogs:
				ld := gen.Logs(batchSize)
				pushes = append(pushes, func() error { return e.PushLogs(ctx, ld) })
			case pipeline.SignalTraces:
				td := gen.Traces(batchSize)
				pushes = append(pushes, func() error { return e.PushTraces(ctx, td) })
			default:
				md := gen.Metrics(batchSize)
				pushes = append(pushes, func() error { return e.PushMetrics(ctx, md) })
			}
		}
		// The garbage collector is paused while the pushes are timed: its cost
		// follows the live heap of the test process, which is not the heap of
		// a collector. Allocated bytes are reported instead.
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		gcPercent := debug.SetGCPercent(-1)
		cpuBefore := cpuTime()
		start := time.Now()
		for _, push := range pushes {
			require.NoError(t, push())
		}
		res.wall += time.Since(start)
		res.cpu += cpuTime() - cpuBefore
		debug.SetGCPercent(gcPercent)
		runtime.ReadMemStats(&after)
		res.allocated += after.TotalAlloc - before.TotalAlloc
	}
	var after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&after)
	res.heapAfter = after.HeapAlloc
	require.NoError(t, e.Shutdown(ctx))

	res.attributesRows = countRows(t, conn, "SELECT count() FROM signoz_metadata.attributes_metadata")
	res.fieldValuesRows = countRows(t, conn, "SELECT count() FROM signoz_metadata.field_values_sets")
	require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH LOGS"))
	require.NoError(t, conn.QueryRow(ctx, `SELECT count() FROM system.query_log
WHERE type = 'QueryFinish' AND query_kind = 'Insert' AND is_initial_query AND event_time >= ?
  AND has(tables, 'signoz_metadata.distributed_field_values_sets')`, since).Scan(&res.fieldValuesInserts))
	require.NoError(t, conn.QueryRow(ctx, `SELECT count() FROM system.query_log
WHERE type = 'QueryFinish' AND query_kind = 'Insert' AND is_initial_query AND event_time >= ?
  AND has(tables, 'signoz_metadata.distributed_attributes_metadata')`, since).Scan(&res.attributesInserts))
	var attrsMillis, fvMillis uint64
	require.NoError(t, conn.QueryRow(ctx, `SELECT
    sumIf(query_duration_ms, arrayExists(x -> x LIKE 'signoz_metadata.%attributes_metadata', tables)),
    sumIf(query_duration_ms, arrayExists(x -> x LIKE 'signoz_metadata.%field_values%', tables))
FROM system.query_log
WHERE type = 'QueryFinish' AND query_kind = 'Insert' AND event_time >= ?`, since).Scan(&attrsMillis, &fvMillis))
	res.attributesServerCPU = time.Duration(attrsMillis) * time.Millisecond
	res.fieldValuesServerCPU = time.Duration(fvMillis) * time.Millisecond
	return res
}

func truncate(t *testing.T, conn driver.Conn) {
	t.Helper()
	for _, table := range []string{"attributes_metadata", "field_values_sets", "field_values_daily"} {
		require.NoError(t, conn.Exec(context.Background(), "TRUNCATE TABLE signoz_metadata."+table))
	}
}

func cpuTime() time.Duration {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func bytesString(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

type perfQuery struct {
	name, sql string
}

var perfQueries = []perfQuery{
	{"plain values: logs http.route (top 50 by holders)", `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute' AND day = toDate(now(), 'UTC')
GROUP BY string_value ORDER BY uniqHLL12Merge(holders) DESC LIMIT 51`},
	{"plain values: logs user.id (high-cardinality sample)", `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'user.id' AND field_context = 'attribute' AND day = toDate(now(), 'UTC')
GROUP BY string_value ORDER BY uniqHLL12Merge(holders) DESC LIMIT 51`},
	{"type-ahead: logs http.route ILIKE '%route-1%'", `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute' AND day = toDate(now(), 'UTC') AND string_value ILIKE '%route-1%'
GROUP BY string_value ORDER BY uniqHLL12Merge(holders) DESC LIMIT 51`},
	{"related values: logs http.route where service.name = svc-01 and http.method = GET", `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute'
  AND last_seen >= toDateTime(toDate(now(), 'UTC'), 'UTC')
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'logs' AND source = '' AND field_name = 'service.name' AND field_context = 'resource' AND string_value = 'svc-01')
  AND (resource_hash, attrs_hash) IN (SELECT resource_hash, attrs_hash FROM signoz_metadata.distributed_field_values_sets AS c
      WHERE signal = 'logs' AND source = '' AND field_name = 'http.method' AND field_context = 'attribute' AND string_value = 'GET')
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51
SETTINGS distributed_product_mode = 'local'`},
	{"related values (old store, same question): attributes_metadata", `SELECT DISTINCT attributes['http.route'] FROM signoz_metadata.distributed_attributes_metadata
WHERE data_source = 'logs' AND resource_attributes['service.name'] = 'svc-01' AND attributes['http.method'] = 'GET'
  AND unix_milli >= toUnixTimestamp(toDate(now(), 'UTC')) * 1000 LIMIT 51`},
	{"related values: traces name where has_error = true", `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'traces' AND source = '' AND metric_name = '' AND field_name = 'name' AND field_context = 'span'
  AND (resource_hash, attrs_hash) IN (SELECT resource_hash, attrs_hash FROM signoz_metadata.distributed_field_values_sets AS c
      WHERE signal = 'traces' AND source = '' AND field_name = 'has_error' AND field_context = 'span' AND string_value = 'true')
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51
SETTINGS distributed_product_mode = 'local'`},
	{"metric keys: app_metric_01", `SELECT field_context, field_name FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'metrics' AND source = '' AND metric_name = 'app_metric_01' AND day = toDate(now(), 'UTC')
GROUP BY field_context, field_name`},
	{"metric label values: app_metric_01 route where service.name = svc-01", `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'metrics' AND source = '' AND metric_name = 'app_metric_01' AND field_name = 'route' AND field_context = 'attribute'
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'metrics' AND source = '' AND metric_name = 'app_metric_01' AND field_name = 'service.name' AND field_context = 'resource' AND string_value = 'svc-01')
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51
SETTINGS distributed_product_mode = 'local'`},
}

func readReport(t *testing.T, conn driver.Conn) string {
	t.Helper()
	ctx := context.Background()
	var out strings.Builder
	fmt.Fprintf(&out, "| read | median time (5 runs) | rows read | values |\n|---|---|---|---|\n")
	for i, q := range perfQueries {
		comment := fmt.Sprintf("fieldvalues-perf-%d", i)
		var durations []time.Duration
		var values int
		for run := 0; run < 5; run++ {
			start := time.Now()
			rows, err := conn.Query(clickhouseContext(ctx, comment), q.sql)
			require.NoError(t, err, q.sql)
			values = 0
			for rows.Next() {
				values++
			}
			require.NoError(t, rows.Err())
			_ = rows.Close()
			durations = append(durations, time.Since(start))
		}
		sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
		require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH LOGS"))
		var readRows uint64
		require.NoError(t, conn.QueryRow(ctx, `SELECT max(read_rows) FROM system.query_log
WHERE type = 'QueryFinish' AND log_comment = ? AND is_initial_query`, comment).Scan(&readRows))
		fmt.Fprintf(&out, "| %s | %s | %d | %d |\n", q.name, durations[2].Round(100*time.Microsecond), readRows, values)
	}
	return out.String()
}

func sizeReport(t *testing.T, conn driver.Conn) string {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"attributes_metadata", "field_values_sets", "field_values_daily"} {
		require.NoError(t, conn.Exec(ctx, "OPTIMIZE TABLE signoz_metadata."+table+" FINAL"))
	}
	rows, err := conn.Query(ctx, `SELECT table, sum(rows), formatReadableSize(sum(bytes_on_disk))
FROM system.parts WHERE database = 'signoz_metadata' AND active AND table IN ('attributes_metadata', 'field_values_sets', 'field_values_daily')
GROUP BY table ORDER BY table`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out strings.Builder
	fmt.Fprintf(&out, "| table, after merges | rows | on disk |\n|---|---|---|\n")
	for rows.Next() {
		var table, size string
		var n uint64
		require.NoError(t, rows.Scan(&table, &n, &size))
		fmt.Fprintf(&out, "| %s | %d | %s |\n", table, n, size)
	}
	return out.String()
}

func clickhouseContext(ctx context.Context, comment string) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"log_comment": comment}))
}
