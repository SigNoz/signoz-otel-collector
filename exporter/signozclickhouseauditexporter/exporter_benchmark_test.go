package signozclickhouseauditexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// discardConn accepts every batch without a ClickHouse server, so the benchmark measures only the exporter.
type discardConn struct{ driver.Conn }

func (discardConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return discardBatch{}, nil
}

type discardBatch struct{ driver.Batch }

func (discardBatch) Append(...any) error { return nil }
func (discardBatch) Send() error         { return nil }
func (discardBatch) Close() error        { return nil }

func benchmarkAuditLogs(resources int, recordsPerResource int) plog.Logs {
	ld := plog.NewLogs()
	start := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	for i := range resources {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "signoz")
		rl.Resource().Attributes().PutStr("service.version", "0.91.0")
		rl.Resource().Attributes().PutStr("signoz.audit.resource.kind", "dashboard")
		rl.Resource().Attributes().PutStr("signoz.audit.resource.id", fmt.Sprintf("dashboard-%d", i))

		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("signoz.audit")
		sl.Scope().SetVersion("1.0.0")

		for j := range recordsPerResource {
			record := sl.LogRecords().AppendEmpty()
			record.SetTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Duration(i*recordsPerResource+j) * time.Millisecond)))
			record.SetSeverityText("INFO")
			record.SetSeverityNumber(plog.SeverityNumberInfo)
			record.SetEventName("dashboard.updated")
			record.Body().SetStr(fmt.Sprintf("user%d@corp.net updated dashboard dashboard-%d", j, i))

			attrs := record.Attributes()
			attrs.PutStr("signoz.audit.principal.id", fmt.Sprintf("principal-%d", j))
			attrs.PutStr("signoz.audit.principal.type", "user")
			attrs.PutStr("signoz.audit.principal.email", fmt.Sprintf("user%d@corp.net", j))
			attrs.PutStr("signoz.audit.action", "update")
			attrs.PutStr("signoz.audit.action_category", "configuration_change")
			attrs.PutStr("signoz.audit.outcome", "success")
			attrs.PutStr("signoz.audit.change.from", `{"title":"dashboard","widgets":2}`)
			attrs.PutStr("signoz.audit.change.to", `{"title":"dashboard v2","widgets":3}`)
			attrs.PutStr("http.request.method", "PUT")
			attrs.PutStr("http.route", "/api/v1/dashboards/{id}")
			attrs.PutInt("http.response.status_code", 200)
			attrs.PutStr("client.address", "10.0.92.243")
		}
	}

	return ld
}

func BenchmarkPushLogsData(b *testing.B) {
	exp := newTestExporter(b)
	exp.conn = discardConn{}
	ld := benchmarkAuditLogs(100, 100)

	b.ReportAllocs()

	for b.Loop() {
		exp.rfCache.DeleteAll()
		require.NoError(b, exp.pushLogsData(context.Background(), ld))
	}
}
