package metadataexporter

import (
	"context"
	"os"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
)

// The integration test needs a ClickHouse server with a cluster for the
// distributed tables. It recreates the signoz_metadata database.
//
//	FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:19000 FIELDVALUES_CLICKHOUSE_CLUSTER=c1 go test -run Integration ./exporter/metadataexporter/
func integrationSchema(t *testing.T) (driver.Conn, string) {
	t.Helper()
	dsn := os.Getenv("FIELDVALUES_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("set FIELDVALUES_CLICKHOUSE_DSN to run the ClickHouse integration test")
	}
	cluster := os.Getenv("FIELDVALUES_CLICKHOUSE_CLUSTER")
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
	for _, migration := range schemamigrator.MetadataMigrations {
		if migration.MigrationID != 1000 && migration.MigrationID != 1002 {
			continue
		}
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
	}
	return conn, dsn
}

func countRows(t *testing.T, conn driver.Conn, query string) uint64 {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH DISTRIBUTED signoz_metadata.distributed_field_values_sets"))
	require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH DISTRIBUTED signoz_metadata.distributed_attributes_metadata"))
	var n uint64
	require.NoError(t, conn.QueryRow(ctx, query).Scan(&n))
	return n
}

// Both writers run when field_values is on: the attribute writer of today
// and the field values writer.
func TestIntegrationExporterWritesBothStores(t *testing.T) {
	conn, dsn := integrationSchema(t)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DSN = dsn
	cfg.Enabled = true
	cfg.FieldValues.Enabled = true
	cfg.FieldValues.Cache.MaxBytes = 1 << 20
	set := exportertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = componenttest.NewNopTelemetrySettings()
	ctx := context.Background()
	now := pcommon.NewTimestampFromTime(time.Now())

	logsExp, err := factory.CreateLogs(ctx, set, cfg)
	require.NoError(t, err)
	tracesExp, err := factory.CreateTraces(ctx, set, cfg)
	require.NoError(t, err)
	metricsExp, err := factory.CreateMetrics(ctx, set, cfg)
	require.NoError(t, err)
	require.NoError(t, logsExp.Start(ctx, componenttest.NewNopHost()))
	require.NoError(t, tracesExp.Start(ctx, componenttest.NewNopHost()))
	require.NoError(t, metricsExp.Start(ctx, componenttest.NewNopHost()))

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(now)
	lr.Attributes().PutStr("http.method", "GET")
	require.NoError(t, logsExp.ConsumeLogs(ctx, ld))

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("GET /cart")
	span.SetStartTimestamp(now)
	span.Attributes().PutStr("http.method", "GET")
	require.NoError(t, tracesExp.ConsumeTraces(ctx, td))

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("http_requests")
	m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.Attributes().PutStr("code", "200")
	require.NoError(t, metricsExp.ConsumeMetrics(ctx, md))

	require.NoError(t, logsExp.Shutdown(ctx))
	require.NoError(t, tracesExp.Shutdown(ctx))
	require.NoError(t, metricsExp.Shutdown(ctx))

	for _, signal := range []string{"logs", "traces", "metrics"} {
		assert.Positive(t, countRows(t, conn, "SELECT count() FROM signoz_metadata.distributed_attributes_metadata WHERE data_source = '"+signal+"'"),
			"%s: the attribute writer of today still writes", signal)
		assert.Positive(t, countRows(t, conn, "SELECT count() FROM signoz_metadata.distributed_field_values_sets WHERE signal = '"+signal+"'"),
			"%s: the field values writer writes", signal)
	}
}
