package fieldvalues

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvaluestest"
)

// TestPerfReadsOverDays writes several days of logs, one UTC day after the
// other, and times the reads for a window of one day and of all days.
//
//	FIELDVALUES_PERF=1 FIELDVALUES_PERF_DAYS=7 FIELDVALUES_PERF_BATCHES=150 \
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 \
//	go test -run TestPerfReadsOverDays -v -timeout 60m ./exporter/metadataexporter/internal/fieldvalues/
func TestPerfReadsOverDays(t *testing.T) {
	if os.Getenv("FIELDVALUES_PERF") == "" {
		t.Skip("set FIELDVALUES_PERF=1 to run the load test")
	}
	conn := integrationConn(t)
	days, batches := 7, 150
	if v := os.Getenv("FIELDVALUES_PERF_DAYS"); v != "" {
		_, err := fmt.Sscan(v, &days)
		require.NoError(t, err)
	}
	if v := os.Getenv("FIELDVALUES_PERF_BATCHES"); v != "" {
		_, err := fmt.Sscan(v, &batches)
		require.NoError(t, err)
	}
	cfg := testConfig()
	cfg.Cache.MaxBytes = 64 << 20
	w := newIntegrationExporter(t, conn, cfg, pipeline.SignalLogs)
	last := time.Now().UTC().Truncate(24 * time.Hour).Add(12 * time.Hour)
	for d := days - 1; d >= 0; d-- {
		day := last.Add(-time.Duration(d) * 24 * time.Hour)
		setNow(w, day)
		gen := fieldvaluestest.NewGenerator(int64(d), day)
		gen.Correlated = true
		for i := 0; i < batches; i++ {
			require.NoError(t, w.WriteLogs(context.Background(), gen.Logs(1000)))
		}
	}
	flush(t, conn)
	ctx := context.Background()
	var rows uint64
	require.NoError(t, conn.QueryRow(ctx, "SELECT count() FROM signoz_metadata.field_values_sets").Scan(&rows))
	t.Logf("%d days x %d batches of 1,000 log records: %d rows in field_values_sets before merges", days, batches, rows)

	related := `SELECT string_value FROM signoz_metadata.distributed_field_values_sets AS v
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute'
  AND first_seen < fromUnixTimestamp(?) AND last_seen >= fromUnixTimestamp(?)
  AND resource_hash IN (SELECT resource_hash FROM signoz_metadata.distributed_field_values_sets AS r
      WHERE signal = 'logs' AND source = '' AND field_name = 'service.name' AND field_context = 'resource' AND string_value = 'svc-01')
  AND (resource_hash, attrs_hash) IN (SELECT resource_hash, attrs_hash FROM signoz_metadata.distributed_field_values_sets AS c
      WHERE signal = 'logs' AND source = '' AND field_name = 'http.method' AND field_context = 'attribute' AND string_value = 'GET')
GROUP BY string_value ORDER BY uniq(resource_hash, attrs_hash) DESC LIMIT 51
SETTINGS distributed_product_mode = 'local'`
	plain := `SELECT string_value FROM signoz_metadata.distributed_field_values_daily
WHERE signal = 'logs' AND source = '' AND metric_name = '' AND field_name = 'http.route' AND field_context = 'attribute'
  AND day >= toDate(fromUnixTimestamp64Milli(?), 'UTC') AND day < toDate(fromUnixTimestamp64Milli(?), 'UTC') + 1
GROUP BY string_value ORDER BY uniqHLL12Merge(holders) DESC LIMIT 51`

	end := last.Add(12 * time.Hour).UnixMilli()
	for _, window := range []int{1, days} {
		startDay := last.Truncate(24*time.Hour).AddDate(0, 0, -(window - 1)).UnixMilli()
		for _, q := range []struct {
			name, sql string
			args      []any
		}{
			{"related values", related, []any{uint64(end / 1000), uint64(startDay / 1000)}},
			{"plain values", plain, []any{startDay, end - 1}},
		} {
			comment := fmt.Sprintf("fieldvalues-days-%s-%d", q.name, window)
			var durations []time.Duration
			for run := 0; run < 5; run++ {
				start := time.Now()
				r, err := conn.Query(clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"log_comment": comment})), q.sql, q.args...)
				require.NoError(t, err)
				for r.Next() {
				}
				require.NoError(t, r.Err())
				_ = r.Close()
				durations = append(durations, time.Since(start))
			}
			sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
			require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH LOGS"))
			var readRows uint64
			require.NoError(t, conn.QueryRow(ctx, `SELECT max(read_rows) FROM system.query_log WHERE type = 'QueryFinish' AND log_comment = ? AND is_initial_query`, comment).Scan(&readRows))
			t.Logf("%s, window of %d day(s): median %s, %d rows read", q.name, window, durations[2].Round(100*time.Microsecond), readRows)
		}
	}
}
