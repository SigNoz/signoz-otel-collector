package fieldvalues

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvaluestest"
)

// metricRowCounter counts the metric rows of each day by kind.
type metricRowCounter struct {
	next  rowWriter
	today metricDay
	days  []metricDay
}

type metricDay struct {
	series, resource, link, key int
}

func (c *metricRowCounter) write(ctx context.Context, rows []row) error {
	for _, r := range rows {
		switch {
		case r.attrsHash != resourceAttrsHash:
			c.today.series++
		case r.p.name == linkField:
			c.today.link++
		case r.metricName != "":
			c.today.key++
		default:
			c.today.resource++
		}
	}
	return c.next.write(ctx, rows)
}

func (c *metricRowCounter) endDay() {
	c.days = append(c.days, c.today)
	c.today = metricDay{}
}

func envInt(t *testing.T, name string, def int) int {
	t.Helper()
	if v := os.Getenv(name); v != "" {
		n, err := strconv.Atoi(v)
		require.NoError(t, err)
		return n
	}
	return def
}

// churnLayouts are the pair table, partitioned by week, and two copies of it
// that get every insert: one with no partitions and one partitioned by day.
var churnLayouts = []struct{ name, table, partition string }{
	{"weekly partitions (the table)", "field_values_sets", "toMonday(last_seen)"},
	{"no partitions", "field_values_sets_flat", ""},
	{"daily partitions", "field_values_sets_by_day", "toDate(last_seen)"},
}

// TestPerfChurn writes days of Kubernetes-shaped logs and metrics, where the
// pods of a share of the deployments are new each day, and measures the costs
// that a test with the same pods every day cannot show:
//
//   - The pair table has no time in its sort key, so a read reads the rows of
//     every set in its partitions. The test compares the rows read for a day
//     and a week with the rows of the sets active in them, for the weekly
//     partitions of the table, no partitions, and daily partitions, and the
//     merge work and TTL of each.
//   - Metric resource rows are written once per resource, with link and key
//     rows per metric. The test counts each kind of row, and times the reads
//     that use them: the values of a resource key for a metric, the metrics of
//     a resource, and the keys of a group of metrics.
//
// Run it with:
//
//	FIELDVALUES_PERF=1 FIELDVALUES_CHURN_DAYS=35 FIELDVALUES_CHURN_PODS=300 FIELDVALUES_CHURN_PERCENT=20 \
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 \
//	go test -run TestPerfChurn -v -timeout 60m ./exporter/metadataexporter/internal/fieldvalues/
func TestPerfChurn(t *testing.T) {
	if os.Getenv("FIELDVALUES_PERF") == "" {
		t.Skip("set FIELDVALUES_PERF=1 to run the load test")
	}
	days := envInt(t, "FIELDVALUES_CHURN_DAYS", 35)
	pods := envInt(t, "FIELDVALUES_CHURN_PODS", 300)
	percent := envInt(t, "FIELDVALUES_CHURN_PERCENT", 20)
	conn := integrationConn(t)
	ctx := context.Background()
	began := time.Now()
	run := fmt.Sprint(began.Unix())

	queries := []string{"SYSTEM STOP TTL MERGES signoz_metadata.field_values_sets"}
	for _, l := range churnLayouts[1:] {
		// CREATE TABLE AS keeps the partition key of the table unless the
		// copy sets its own.
		engine := "ENGINE = AggregatingMergeTree PARTITION BY tuple()"
		settings := "SETTINGS allow_nullable_key = 1"
		if l.partition != "" {
			engine = "ENGINE = AggregatingMergeTree PARTITION BY " + l.partition
			settings += ", ttl_only_drop_parts = 1"
		}
		queries = append(queries,
			`CREATE TABLE signoz_metadata.`+l.table+` AS signoz_metadata.field_values_sets `+engine+`
ORDER BY (signal, source, metric_name, field_name, field_context, field_data_type, string_value, number_value, resource_hash, attrs_hash)
TTL last_seen + toIntervalDay(30) `+settings,
			`CREATE MATERIALIZED VIEW signoz_metadata.`+l.table+`_mv TO signoz_metadata.`+l.table+`
AS SELECT * FROM signoz_metadata.field_values_sets`,
			"SYSTEM STOP TTL MERGES signoz_metadata."+l.table,
		)
	}
	for _, q := range queries {
		require.NoError(t, conn.Exec(ctx, q), q)
	}

	cfg := testConfig()
	cfg.Cache.MaxBytes = 256 << 20
	logs := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	metrics := newIntegrationExporter(t, conn, cfg, pipeline.SignalMetrics)
	counter := &metricRowCounter{next: metrics.rows}
	metrics.rows = counter

	cluster := fieldvaluestest.NewCluster(7, pods)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	first := today.AddDate(0, 0, -days)
	newPods := 0
	for d := 0; d < days; d++ {
		if d > 0 {
			newPods += cluster.Roll(float64(percent) / 100)
		}
		day := first.AddDate(0, 0, d)
		setNow(logs, day.Add(12*time.Hour))
		setNow(metrics, day.Add(12*time.Hour))
		for _, ld := range cluster.Logs(day.Add(11*time.Hour), 100, 1000) {
			require.NoError(t, logs.WriteLogs(ctx, ld))
		}
		for _, md := range cluster.Metrics(day.Add(11*time.Hour), 1000) {
			require.NoError(t, metrics.WriteMetrics(ctx, md))
		}
		counter.endDay()
	}
	flush(t, conn)
	t.Logf("%d days, %d pods, %d%% of deployments roll each day: %d new pods in all, written in %s",
		days, cluster.Pods(), percent, newPods, time.Since(began).Round(time.Second))

	var report strings.Builder
	fmt.Fprintf(&report, "\nMerges during the load, and parts before OPTIMIZE\n")
	fmt.Fprintf(&report, "| layout | merges | bytes read by merges | bytes written by merges | active parts |\n|---|---|---|---|---|\n")
	require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH LOGS"))
	for _, l := range churnLayouts {
		var merges, read, written, parts uint64
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(), sum(read_bytes), sum(size_in_bytes),
    (SELECT count() FROM system.parts WHERE active AND database = 'signoz_metadata' AND table = ?)
FROM system.part_log WHERE event_type = 'MergeParts' AND database = 'signoz_metadata' AND table = ? AND event_time >= fromUnixTimestamp(?)`,
			l.table, l.table, began.Unix()).Scan(&merges, &read, &written, &parts))
		fmt.Fprintf(&report, "| %s | %d | %s | %s | %d |\n", l.name, merges, formatBytes(read), formatBytes(written), parts)
	}

	for _, table := range []string{"field_values_sets", "field_values_sets_flat", "field_values_sets_by_day", "field_values_daily", "field_keys_daily"} {
		require.NoError(t, conn.Exec(ctx, "OPTIMIZE TABLE signoz_metadata."+table+" FINAL"))
	}

	end := today.AddDate(0, 0, -1).Add(12 * time.Hour)
	fmt.Fprintf(&report, "\nRows read by related-value reads that end at %s\n", end.Format(time.DateTime))
	fmt.Fprintf(&report, "| read | window | rows of active sets |")
	for _, l := range churnLayouts {
		fmt.Fprintf(&report, " rows read, %s |", l.name)
	}
	fmt.Fprintf(&report, "\n|---|---|---|---|---|---|\n")
	for _, q := range churnReads {
		for _, window := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
			from := end.Add(-window)
			active := countRows(t, conn, q.active, from.Unix(), end.Unix())
			var read []string
			for _, l := range churnLayouts {
				n := readRows(t, conn, fmt.Sprintf("churn-%s-%s-%s-%s", q.name, window, l.table, run), strings.ReplaceAll(q.sql, "{table}", l.table), from.Unix(), end.Unix())
				read = append(read, fmt.Sprint(n))
			}
			fmt.Fprintf(&report, "| %s | %s | %d | %s |\n", q.name, window, active, strings.Join(read, " | "))
		}
	}

	fmt.Fprintf(&report, "\nSize and TTL, before any TTL merge\n")
	fmt.Fprintf(&report, "| layout | rows | on disk | partitions | rows past TTL | bytes a TTL merge rewrites | bytes TTL drops whole |\n|---|---|---|---|---|---|---|\n")
	for _, l := range churnLayouts {
		var rows, bytes, rewrite, drop, partitions uint64
		require.NoError(t, conn.QueryRow(ctx, `SELECT sum(rows), sum(bytes_on_disk),
    sumIf(bytes_on_disk, delete_ttl_info_min <= now() AND delete_ttl_info_max > now()),
    sumIf(bytes_on_disk, delete_ttl_info_max <= now()),
    uniqExact(partition)
FROM system.parts WHERE active AND database = 'signoz_metadata' AND table = ?`, l.table).Scan(&rows, &bytes, &rewrite, &drop, &partitions))
		expired := countRows(t, conn, "SELECT count() FROM signoz_metadata."+l.table+" WHERE last_seen + toIntervalDay(30) <= now()")
		if l.partition != "" {
			// With ttl_only_drop_parts, a part with some rows past TTL waits
			// until all its rows are past TTL; nothing is rewritten.
			rewrite = 0
		}
		fmt.Fprintf(&report, "| %s | %d | %s | %d | %d | %s | %s |\n", l.name, rows, formatBytes(bytes), partitions, expired, formatBytes(rewrite), formatBytes(drop))
	}

	n := min(7, len(counter.days))
	var mean metricDay
	for _, d := range counter.days[len(counter.days)-n:] {
		mean.series += d.series / n
		mean.resource += d.resource / n
		mean.link += d.link / n
		mean.key += d.key / n
	}
	fmt.Fprintf(&report, "\nMetric rows per day, mean of the last %d days\n| series | resource | link | key |\n|---|---|---|---|\n| %d | %d | %d | %d |\n",
		n, mean.series, mean.resource, mean.link, mean.key)

	reportMetricReads(t, conn, &report, end, run)
	t.Log(report.String())
}

type churnRead struct {
	name, sql, active string
}

// The reads filter on a resource field and an attribute, as a quick filter
// does. active counts, on the copy with no partitions, the rows of the same
// ranges whose sets were seen in the window.
var churnReads = []churnRead{
	{
		name: "logs http.route where k8s.namespace.name = ns-01 and severity_text = ERROR",
		sql: `SELECT string_value FROM signoz_metadata.{table}
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute'
  AND first_seen < fromUnixTimestamp($2) AND last_seen >= fromUnixTimestamp($1)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.{table}
      WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'k8s.namespace.name' AND field_context = 'resource'
        AND string_value = 'ns-01' AND last_seen >= fromUnixTimestamp($1))
  AND (resource_hash, attrs_hash) IN (SELECT resource_hash, attrs_hash FROM signoz_metadata.{table}
      WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'severity_text' AND field_context = 'log'
        AND string_value = 'ERROR' AND last_seen >= fromUnixTimestamp($1))
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51`,
		active: `SELECT count() FROM signoz_metadata.field_values_sets_flat FINAL
WHERE signal = 'logs' AND source = '' AND metric_name = ''
  AND ((field_name = 'http.route' AND field_context = 'attribute')
    OR (field_name = 'k8s.namespace.name' AND field_context = 'resource' AND string_value = 'ns-01')
    OR (field_name = 'severity_text' AND field_context = 'log' AND string_value = 'ERROR'))
  AND last_seen >= fromUnixTimestamp(?) AND first_seen < fromUnixTimestamp(?)`,
	},
	{
		name: "metric http.server.request.duration.count http.route where k8s.namespace.name = ns-01",
		sql: `SELECT string_value FROM signoz_metadata.{table}
WHERE signal = 'metrics' AND source = '' AND metric_name = 'http.server.request.duration.count' AND field_name = 'http.route' AND field_context = 'attribute'
  AND first_seen < fromUnixTimestamp($2) AND last_seen >= fromUnixTimestamp($1)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.{table}
      WHERE signal = 'metrics' AND source = '' AND metric_name = '' AND field_name = 'k8s.namespace.name'
        AND field_context = 'resource' AND string_value = 'ns-01' AND last_seen >= fromUnixTimestamp($1))
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51`,
		active: `SELECT count() FROM signoz_metadata.field_values_sets_flat FINAL
WHERE signal = 'metrics' AND source = ''
  AND ((metric_name = 'http.server.request.duration.count' AND field_name = 'http.route' AND field_context = 'attribute')
    OR (metric_name = '' AND field_name = 'k8s.namespace.name' AND field_context = 'resource' AND string_value = 'ns-01'))
  AND last_seen >= fromUnixTimestamp(?) AND first_seen < fromUnixTimestamp(?)`,
	},
}

// reportMetricReads times the reads that use link and key rows.
func reportMetricReads(t *testing.T, conn driver.Conn, report *strings.Builder, end time.Time, run string) {
	from := end.Add(-7 * 24 * time.Hour).Unix()
	fmt.Fprintf(report, "\nMetric reads over the last 7 days (median of 5)\n| read | results | rows read | time |\n|---|---|---|---|\n")
	for _, q := range []struct {
		name, sql string
		args      []any
	}{
		{"values of k8s.namespace.name for k8s.pod.metric_00", metricResourceValues, []any{"k8s.namespace.name", from, from}},
		{"values of k8s.pod.name for k8s.pod.metric_00", metricResourceValues, []any{"k8s.pod.name", from, from}},
		{"metrics where service.name = deploy-001", `SELECT string_value FROM signoz_metadata.field_values_sets
WHERE signal = 'metrics' AND source = '' AND metric_name = '' AND field_name = '__name__' AND last_seen >= fromUnixTimestamp($1)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.field_values_sets
      WHERE signal = 'metrics' AND source = '' AND metric_name = '' AND field_name = 'service.name' AND string_value = 'deploy-001')
GROUP BY string_value LIMIT 1001`, []any{from}},
		{"keys of k8s.* (issue 13042)", `SELECT field_name, field_context FROM signoz_metadata.field_keys_daily
WHERE signal = 'metrics' AND source = '' AND metric_name LIKE 'k8s.%' AND day >= toDate(fromUnixTimestamp($1), 'UTC')
GROUP BY field_name, field_context LIMIT 1001`, []any{from}},
	} {
		comment := fmt.Sprintf("churn-metric-%s-%s", q.name, run)
		results := countRows(t, conn, "SELECT count() FROM ("+q.sql+")", q.args...)
		read := readRows(t, conn, comment, q.sql, q.args...)
		fmt.Fprintf(report, "| %s | %d | %d | %s |\n", q.name, results, read, readTime(t, conn, comment))
	}
}

const metricResourceValues = `SELECT string_value FROM signoz_metadata.field_values_sets
WHERE signal = 'metrics' AND source = '' AND metric_name = '' AND field_name = $1 AND field_context = 'resource' AND last_seen >= fromUnixTimestamp($2)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.field_values_sets
      WHERE signal = 'metrics' AND source = '' AND metric_name = '' AND field_name = '__name__' AND string_value = 'k8s.pod.metric_00'
        AND last_seen >= fromUnixTimestamp($3))
GROUP BY string_value ORDER BY uniq(resource_hash) DESC LIMIT 51`

func countRows(t *testing.T, conn driver.Conn, query string, args ...any) uint64 {
	t.Helper()
	var n uint64
	require.NoError(t, conn.QueryRow(context.Background(), query, args...).Scan(&n), query)
	return n
}

// readRows runs a read five times and returns the rows it read, from the
// query log.
func readRows(t *testing.T, conn driver.Conn, comment, query string, args ...any) uint64 {
	t.Helper()
	ctx := clickhouse.Context(context.Background(), clickhouse.WithSettings(clickhouse.Settings{"log_comment": comment}))
	for run := 0; run < 5; run++ {
		rows, err := conn.Query(ctx, query, args...)
		require.NoError(t, err, query)
		for rows.Next() {
		}
		require.NoError(t, rows.Err())
		_ = rows.Close()
	}
	require.NoError(t, conn.Exec(context.Background(), "SYSTEM FLUSH LOGS"))
	return countRows(t, conn, `SELECT max(read_rows) FROM system.query_log WHERE type = 'QueryFinish' AND log_comment = ?`, comment)
}

func readTime(t *testing.T, conn driver.Conn, comment string) time.Duration {
	t.Helper()
	var ms []uint64
	rows, err := conn.Query(context.Background(), `SELECT query_duration_ms FROM system.query_log WHERE type = 'QueryFinish' AND log_comment = ?`, comment)
	require.NoError(t, err)
	for rows.Next() {
		var v uint64
		require.NoError(t, rows.Scan(&v))
		ms = append(ms, v)
	}
	require.NoError(t, rows.Close())
	sort.Slice(ms, func(a, b int) bool { return ms[a] < ms[b] })
	return time.Duration(ms[len(ms)/2]) * time.Millisecond
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}
