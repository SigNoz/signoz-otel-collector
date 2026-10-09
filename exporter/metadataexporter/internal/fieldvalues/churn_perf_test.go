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

// metricRowCounter counts the metric rows of each day, and the rows that the
// layout with link rows would write for the same data: each resource row once
// with no metric, one link row per metric and resource, and one key row per
// metric and resource key.
type metricRowCounter struct {
	next rowWriter

	seriesRows, resourceRows int
	resourceOnce             map[string]struct{}
	links, keys              map[string]struct{}
	days                     []metricDay
}

type metricDay struct {
	seriesRows, resourceRows, linkLayoutRows int
}

func (c *metricRowCounter) write(ctx context.Context, rows []row) error {
	for _, r := range rows {
		if r.attrsHash != resourceAttrsHash {
			c.seriesRows++
			continue
		}
		c.resourceRows++
		c.resourceOnce[fmt.Sprintf("%d|%s|%s", r.resourceHash, r.p.name, r.p.str)] = struct{}{}
		c.links[fmt.Sprintf("%s|%d", r.metricName, r.resourceHash)] = struct{}{}
		c.keys[r.metricName+"|"+r.p.name] = struct{}{}
	}
	return c.next.write(ctx, rows)
}

func newMetricRowCounter(next rowWriter) *metricRowCounter {
	c := &metricRowCounter{next: next}
	c.reset()
	return c
}

func (c *metricRowCounter) reset() {
	c.seriesRows, c.resourceRows = 0, 0
	c.resourceOnce, c.links, c.keys = map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
}

func (c *metricRowCounter) endDay() {
	c.days = append(c.days, metricDay{
		seriesRows:     c.seriesRows,
		resourceRows:   c.resourceRows,
		linkLayoutRows: len(c.resourceOnce) + len(c.links) + len(c.keys),
	})
	c.reset()
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

// TestPerfChurn writes days of Kubernetes-shaped logs and metrics, where the
// pods of a share of the deployments are new each day, and measures two costs
// that a test with the same pods every day cannot show:
//
//   - The pair table has no time in its sort key. A read for a short window
//     reads the rows of every set of the last 30 days. The test compares the
//     rows read with the rows of the sets active in the window, on the table
//     and on a copy partitioned by the week of each row.
//
//   - Metric resource rows are written per metric. The test counts them, and
//     the rows of the layout with link rows, and times the read of the values
//     of a resource key for a metric in both layouts.
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

	// Each copy gets every insert of the pair table, so each row goes to the
	// partition of its own time. The key view keeps one row per metric, key
	// and day.
	queries := []string{
		`CREATE TABLE signoz_metadata.field_keys_daily
(signal LowCardinality(String), source LowCardinality(String), metric_name LowCardinality(String), field_context LowCardinality(String),
 field_name LowCardinality(String), field_data_type LowCardinality(String), day Date)
ENGINE = ReplacingMergeTree PARTITION BY toMonday(day) ORDER BY (signal, source, metric_name, field_name, field_context, field_data_type, day)`,
		`CREATE MATERIALIZED VIEW signoz_metadata.field_keys_daily_mv TO signoz_metadata.field_keys_daily AS
SELECT signal, source, scope_metric AS metric_name, toString(field_context) AS field_context, field_name, toString(field_data_type) AS field_data_type,
    toDate(first_seen, 'UTC') AS day
FROM signoz_metadata.field_values_sets
ARRAY JOIN if(field_values_sets.metric_name = '', [''], [field_values_sets.metric_name, '']) AS scope_metric
GROUP BY signal, source, scope_metric, field_context, field_name, field_data_type, day`,
		"SYSTEM STOP TTL MERGES signoz_metadata.field_values_sets",
	}
	for _, l := range churnLayouts[1:] {
		queries = append(queries,
			`CREATE TABLE signoz_metadata.`+l.table+` AS signoz_metadata.field_values_sets
ENGINE = AggregatingMergeTree PARTITION BY `+l.partition+`
ORDER BY (signal, source, metric_name, field_name, field_context, field_data_type, string_value, number_value, resource_hash, attrs_hash)
TTL last_seen + toIntervalDay(30)
SETTINGS allow_nullable_key = 1, ttl_only_drop_parts = 1`,
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
	counter := newMetricRowCounter(metrics.rows)
	metrics.rows = counter

	cluster := fieldvaluestest.NewCluster(7, pods)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	first := today.AddDate(0, 0, -days)
	newPods := 0
	start := time.Now()
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
		days, cluster.Pods(), percent, newPods, time.Since(start).Round(time.Second))

	var report strings.Builder
	fmt.Fprintf(&report, "\nItem 11: merges during the load, and parts before OPTIMIZE\n")
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

	for _, table := range []string{"field_values_sets", "field_values_sets_weekly", "field_values_sets_by_day", "field_values_daily", "field_keys_daily"} {
		require.NoError(t, conn.Exec(ctx, "OPTIMIZE TABLE signoz_metadata."+table+" FINAL"))
	}

	end := today.AddDate(0, 0, -1).Add(12 * time.Hour)
	fmt.Fprintf(&report, "\nItem 11: rows read by related-value reads that end at %s\n", end.Format(time.DateTime))
	fmt.Fprintf(&report, "| read | window | rows of active sets | rows read, no partitions | rows read, weekly | rows read, daily |\n|---|---|---|---|---|---|\n")
	for _, q := range churnReads {
		for _, window := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
			from := end.Add(-window)
			active := countRows(t, conn, q.active, from.Unix(), end.Unix())
			var read []string
			for _, l := range churnLayouts {
				n := readRows(t, conn, fmt.Sprintf("churn-%s-%s-%s-%d", q.name, window, l.table, began.Unix()), strings.ReplaceAll(q.sql, "{table}", l.table), from.Unix(), end.Unix())
				read = append(read, fmt.Sprint(n))
			}
			fmt.Fprintf(&report, "| %s | %s | %d | %s |\n", q.name, window, active, strings.Join(read, " | "))
		}
	}

	fmt.Fprintf(&report, "\nItem 11: size and TTL, before any TTL merge\n")
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
	fmt.Fprintf(&report, "\nItem 10: metric rows per day, mean of the last %d days\n", n)
	var last metricDay
	for _, d := range counter.days[len(counter.days)-n:] {
		last.seriesRows += d.seriesRows / n
		last.resourceRows += d.resourceRows / n
		last.linkLayoutRows += d.linkLayoutRows / n
	}
	fmt.Fprintf(&report, "| series rows | resource rows per metric (now) | resource and link rows (link layout) |\n|---|---|---|\n| %d | %d | %d |\n",
		last.seriesRows, last.resourceRows, last.linkLayoutRows)

	reportLinkLayoutReads(t, conn, &report, end, fmt.Sprint(began.Unix()))
	t.Log(report.String())
}

var churnLayouts = []struct{ name, table, partition string }{
	{"no partitions (now)", "field_values_sets", ""},
	{"weekly partitions", "field_values_sets_weekly", "toMonday(last_seen)"},
	{"daily partitions", "field_values_sets_by_day", "toDate(last_seen)"},
}

type churnRead struct {
	name, sql, active string
}

// The reads filter on a resource field and an attribute, as a quick filter
// does. active counts the rows of the same ranges whose sets were seen in the
// window.
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
		active: `SELECT count() FROM signoz_metadata.field_values_sets FINAL
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
      WHERE signal = 'metrics' AND source = '' AND metric_name = 'http.server.request.duration.count' AND field_name = 'k8s.namespace.name'
        AND field_context = 'resource' AND string_value = 'ns-01' AND last_seen >= fromUnixTimestamp($1))
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51`,
		active: `SELECT count() FROM signoz_metadata.field_values_sets FINAL
WHERE signal = 'metrics' AND source = '' AND metric_name = 'http.server.request.duration.count'
  AND ((field_name = 'http.route' AND field_context = 'attribute')
    OR (field_name = 'k8s.namespace.name' AND field_context = 'resource' AND string_value = 'ns-01'))
  AND last_seen >= fromUnixTimestamp(?) AND first_seen < fromUnixTimestamp(?)`,
	},
}

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

// reportLinkLayoutReads builds the link layout from the metric resource rows
// of the table, and compares the read of the values of a resource key for one
// metric: from the daily view now, and as a join on the link layout.
func reportLinkLayoutReads(t *testing.T, conn driver.Conn, report *strings.Builder, end time.Time, run string) {
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE signoz_metadata.link_resources (field_name LowCardinality(String), string_value String, resource_hash UInt64 CODEC(ZSTD(1)),
    first_seen SimpleAggregateFunction(min, DateTime) CODEC(ZSTD(1)), last_seen SimpleAggregateFunction(max, DateTime) CODEC(ZSTD(1)))
ENGINE = AggregatingMergeTree ORDER BY (field_name, string_value, resource_hash)`,
		`INSERT INTO signoz_metadata.link_resources SELECT field_name, string_value, resource_hash, min(first_seen), max(last_seen)
FROM signoz_metadata.field_values_sets WHERE signal = 'metrics' AND attrs_hash = 0 GROUP BY field_name, string_value, resource_hash`,
		`CREATE TABLE signoz_metadata.link_metrics (metric_name LowCardinality(String), resource_hash UInt64 CODEC(ZSTD(1)),
    first_seen SimpleAggregateFunction(min, DateTime) CODEC(ZSTD(1)), last_seen SimpleAggregateFunction(max, DateTime) CODEC(ZSTD(1)))
ENGINE = AggregatingMergeTree ORDER BY (metric_name, resource_hash)`,
		`INSERT INTO signoz_metadata.link_metrics SELECT metric_name, resource_hash, min(first_seen), max(last_seen)
FROM signoz_metadata.field_values_sets WHERE signal = 'metrics' AND attrs_hash = 0 GROUP BY metric_name, resource_hash`,
		"OPTIMIZE TABLE signoz_metadata.link_resources FINAL",
		"OPTIMIZE TABLE signoz_metadata.link_metrics FINAL",
	} {
		require.NoError(t, conn.Exec(ctx, q), q)
	}

	fmt.Fprintf(report, "\nItem 10: stored metric resource rows after merges\n| layout | rows | on disk |\n|---|---|---|\n")
	var rows, bytes uint64
	require.NoError(t, conn.QueryRow(ctx, `SELECT count(), toUInt64((SELECT sum(bytes_on_disk) FROM system.parts WHERE active AND database = 'signoz_metadata' AND table = 'field_values_sets') * count() /
    (SELECT count() FROM signoz_metadata.field_values_sets))
FROM signoz_metadata.field_values_sets WHERE signal = 'metrics' AND attrs_hash = 0`).Scan(&rows, &bytes))
	fmt.Fprintf(report, "| resource rows per metric (now, bytes pro rata) | %d | %s |\n", rows, formatBytes(bytes))
	require.NoError(t, conn.QueryRow(ctx, `SELECT sum(rows), sum(bytes_on_disk) FROM system.parts
WHERE active AND database = 'signoz_metadata' AND table IN ('link_resources', 'link_metrics')`).Scan(&rows, &bytes))
	fmt.Fprintf(report, "| resource rows once and link rows | %d | %s |\n", rows, formatBytes(bytes))

	fmt.Fprintf(report, "\nItem 10: values of a resource key for k8s.pod.metric_00, last 7 days (median of 5)\n")
	fmt.Fprintf(report, "| key | layout | values | rows read | time |\n|---|---|---|---|---|\n")
	from := end.Add(-7 * 24 * time.Hour)
	for _, key := range []string{"k8s.namespace.name", "k8s.pod.name"} {
		now := `SELECT string_value FROM signoz_metadata.field_values_daily
WHERE signal = 'metrics' AND source = '' AND metric_name = 'k8s.pod.metric_00' AND field_name = ? AND field_context = 'resource'
  AND day >= toDate(fromUnixTimestamp(?), 'UTC')
GROUP BY string_value ORDER BY uniqCombinedMerge(12)(holders) DESC LIMIT 51`
		link := `SELECT string_value FROM signoz_metadata.link_resources
WHERE field_name = ? AND last_seen >= fromUnixTimestamp(?)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.link_metrics
      WHERE metric_name = 'k8s.pod.metric_00' AND last_seen >= fromUnixTimestamp(?))
GROUP BY string_value ORDER BY count() DESC LIMIT 51`
		values := countRows(t, conn, `SELECT uniqExact(string_value) FROM signoz_metadata.link_resources WHERE field_name = ? AND last_seen >= fromUnixTimestamp(?)`, key, from.Unix())
		for _, l := range []struct{ name, sql string }{{"daily view (now)", now}, {"join (link layout)", link}} {
			comment := fmt.Sprintf("churn-values-%s-%s-%s", key, l.name, run)
			args := []any{key, from.Unix()}
			if strings.Contains(l.sql, "link_metrics") {
				args = append(args, from.Unix())
			}
			read := readRows(t, conn, comment, l.sql, args...)
			fmt.Fprintf(report, "| %s | %s | %d | %d | %s |\n", key, l.name, values, read, readTime(t, conn, comment))
		}
	}

	fmt.Fprintf(report, "\nIssue 13042: keys of the metrics k8s.*, all days (median of 5)\n| read | from | keys | rows read | time |\n|---|---|---|---|---|\n")
	for _, q := range []struct{ name, cond string }{
		{"all keys", "field_name ILIKE '%'"},
		{"two exact keys", "field_name IN ('k8s.pod.name', 'direction')"},
	} {
		for _, table := range []string{"field_values_daily", "field_keys_daily"} {
			sql := `SELECT field_name, field_context FROM signoz_metadata.` + table + `
WHERE signal = 'metrics' AND source = '' AND metric_name LIKE 'k8s.%' AND ` + q.cond + `
GROUP BY field_name, field_context LIMIT 1001`
			comment := fmt.Sprintf("churn-keys-%s-%s-%s", q.name, table, run)
			keys := countRows(t, conn, "SELECT count() FROM ("+sql+")")
			read := readRows(t, conn, comment, sql)
			fmt.Fprintf(report, "| %s | %s | %d | %d | %s |\n", q.name, table, keys, read, readTime(t, conn, comment))
		}
	}

	fmt.Fprintf(report, "\nR2: metric names where service.name = deploy-001 (median of 5)\n| layout | metrics | rows read | time |\n|---|---|---|---|\n")
	for _, q := range []struct{ name, sql string }{
		{"pair table (now)", `SELECT DISTINCT metric_name FROM signoz_metadata.field_values_sets
WHERE signal = 'metrics' AND source = '' AND field_name = 'service.name' AND field_context = 'resource' AND string_value = 'deploy-001'`},
		{"daily view (now)", `SELECT DISTINCT metric_name FROM signoz_metadata.field_values_daily
WHERE signal = 'metrics' AND source = '' AND metric_name != '' AND field_name = 'service.name' AND field_context = 'resource' AND string_value = 'deploy-001'`},
		{"link layout", `SELECT DISTINCT metric_name FROM signoz_metadata.link_metrics
WHERE resource_hash IN (SELECT resource_hash FROM signoz_metadata.link_resources WHERE field_name = 'service.name' AND string_value = 'deploy-001')`},
	} {
		comment := fmt.Sprintf("churn-r2-%s-%s", q.name, run)
		metrics := countRows(t, conn, "SELECT count() FROM ("+q.sql+")")
		read := readRows(t, conn, comment, q.sql)
		fmt.Fprintf(report, "| %s | %d | %d | %s |\n", q.name, metrics, read, readTime(t, conn, comment))
	}
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
