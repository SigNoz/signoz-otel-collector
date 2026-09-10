package metadataexporter

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
)

// capturedRow is one row appended to the attributes_metadata insert.
type capturedRow struct {
	unixMilli     int64
	signal        string
	resourceAttrs map[string]string
	attrs         map[string]string
	intrinsics    map[string]string
}

type fakeBatch struct {
	driver.Batch
	conn *fakeConn
}

func (b *fakeBatch) Append(v ...any) error {
	b.conn.rows = append(b.conn.rows, capturedRow{
		unixMilli:     v[0].(int64),
		signal:        v[1].(pipeline.Signal).String(),
		resourceAttrs: v[4].(map[string]string),
		attrs:         v[5].(map[string]string),
		intrinsics:    v[6].(map[string]string),
	})
	return nil
}

func (b *fakeBatch) Send() error  { return nil }
func (b *fakeBatch) Close() error { return nil }

// fakeConn records the rows of every batch prepared on it.
type fakeConn struct {
	driver.Conn
	rows []capturedRow
}

func (c *fakeConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &fakeBatch{conn: c}, nil
}

func (c *fakeConn) Close() error { return nil }

func newTestExporter(t *testing.T, configure func(cfg *Config)) (*metadataExporter, *fakeConn) {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.Enabled = true
	if configure != nil {
		configure(cfg)
	}
	conn := &fakeConn{}
	e, err := newMetadataExporterWithConn(context.Background(), *cfg, exportertest.NewNopSettings(metadata.Type), conn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Shutdown(context.Background()) })
	return e, conn
}

func attrValues(rows []capturedRow, key string) []string {
	values := []string{}
	for _, row := range rows {
		if v, ok := row.attrs[key]; ok {
			values = append(values, v)
		}
	}
	return values
}

func newSpan(t *testing.T, td ptrace.Traces, name string, start time.Time, attrs map[string]any) ptrace.Span {
	t.Helper()
	var rs ptrace.ResourceSpans
	if td.ResourceSpans().Len() == 0 {
		rs = td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "checkout")
		rs.ScopeSpans().AppendEmpty()
	} else {
		rs = td.ResourceSpans().At(0)
	}
	span := rs.ScopeSpans().At(0).Spans().AppendEmpty()
	span.SetName(name)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	require.NoError(t, span.Attributes().FromRaw(attrs))
	return span
}

func TestPushTracesDropsOversizedValuesAndKeepsTheKeyOut(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxStringLength = 16
	})
	now := time.Now()

	td := ptrace.NewTraces()
	newSpan(t, td, "GET /users", now, map[string]any{"payload": strings.Repeat("p", 64), "http.route": "/users"})
	newSpan(t, td, "GET /orders", now, map[string]any{"payload": "short", "http.route": "/orders", "empty": ""})
	require.NoError(t, e.PushTraces(context.Background(), td))

	require.Len(t, conn.rows, 2)
	assert.Empty(t, attrValues(conn.rows, "payload"), "the key is dropped from the row after an oversized value and from later rows with short values")
	assert.Empty(t, attrValues(conn.rows, "empty"), "empty values are not written")
	assert.ElementsMatch(t, []string{"/users", "/orders"}, attrValues(conn.rows, "http.route"))
	assert.ElementsMatch(t, []string{"GET /users", "GET /orders"}, attrValues(conn.rows, "name"))
}

func TestPushTracesDropsKeyPastTheDistinctValueLimit(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxStringDistinctValues = 3
	})
	now := time.Now()

	td := ptrace.NewTraces()
	for _, id := range []string{"r1", "r2", "r3", "r4", "r5"} {
		newSpan(t, td, "GET /users", now, map[string]any{"aws.request_id": id})
	}
	require.NoError(t, e.PushTraces(context.Background(), td))

	assert.ElementsMatch(t, []string{"r1", "r2", "r3"}, attrValues(conn.rows, "aws.request_id"), "values past the limit are not written")
	require.Len(t, conn.rows, 4, "spans without the key collapse into one row")

	td = ptrace.NewTraces()
	newSpan(t, td, "GET /users", now, map[string]any{"aws.request_id": "r1"})
	require.NoError(t, e.PushTraces(context.Background(), td))
	assert.Len(t, conn.rows, 4, "a value seen before the limit is also dropped once the key is over the limit")
}

func TestPushTracesTracksNumericIds(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxStringDistinctValues = 2
	})
	now := time.Now()

	td := ptrace.NewTraces()
	for id := int64(1); id <= 5; id++ {
		newSpan(t, td, "GET /users", now, map[string]any{"task.id": id, "retry": false})
	}
	require.NoError(t, e.PushTraces(context.Background(), td))

	assert.Len(t, conn.rows, 3, "two sets carry a task.id, the rest collapse into one set without it")
}

func TestPushTracesDropsOversizedResourceValues(t *testing.T) {
	arn := "arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/0123456789abcdef0123456789abcdef"
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxResourceStringLength = 32
		cfg.AlwaysIncludeAttributes.Traces = []string{"aws.log.group.arn"}
	})

	td := ptrace.NewTraces()
	newSpan(t, td, "GET /users", time.Now(), nil)
	resource := td.ResourceSpans().At(0).Resource().Attributes()
	resource.PutStr("aws.ecs.task.arn", arn)
	resource.PutStr("aws.log.group.arn", arn)
	require.NoError(t, e.PushTraces(context.Background(), td))

	require.Len(t, conn.rows, 1)
	assert.Equal(t, map[string]string{"service.name": "checkout", "aws.log.group.arn": arn}, conn.rows[0].resourceAttrs)
}

func TestPushTracesClampsTimestampsToTheBucket(t *testing.T) {
	e, conn := newTestExporter(t, nil)
	now := time.Now()
	bucket := e.cfg.MaxDistinctValues.Traces.Bucket
	current := now.UnixMilli() / bucket.Milliseconds() * bucket.Milliseconds()
	earlier := now.Add(-2 * bucket)
	earlierBucket := earlier.UnixMilli() / bucket.Milliseconds() * bucket.Milliseconds()

	td := ptrace.NewTraces()
	newSpan(t, td, "in-range", earlier, map[string]any{"case": "in-range"})
	newSpan(t, td, "unset", time.Time{}, map[string]any{"case": "unset"}).SetStartTimestamp(0)
	newSpan(t, td, "epoch", time.Unix(0, 0), map[string]any{"case": "epoch"})
	newSpan(t, td, "future", now.Add(48*time.Hour), map[string]any{"case": "future"})
	newSpan(t, td, "expired", now.Add(-metadataRetention-time.Hour), map[string]any{"case": "expired"})
	require.NoError(t, e.PushTraces(context.Background(), td))

	got := map[string]int64{}
	for _, row := range conn.rows {
		got[row.attrs["case"]] = row.unixMilli
	}
	assert.Equal(t, map[string]int64{
		"in-range": earlierBucket,
		"unset":    current,
		"epoch":    current,
		"future":   current,
		"expired":  current,
	}, got)
}

func TestPushTracesKeepsKeyOutAfterTheDBCountFallsUnderTheLimit(t *testing.T) {
	e, conn := newTestExporter(t, nil)
	now := time.Now()
	push := func() {
		td := ptrace.NewTraces()
		newSpan(t, td, "GET /users", now, map[string]any{"aws.request_id": "r1", "http.route": "/users"})
		require.NoError(t, e.PushTraces(context.Background(), td))
	}

	e.storeTracesTagValues(map[string]tagValueCountFromDB{
		"aws.request_id": {tagDataType: "string", stringTagValueCount: 5000},
	})
	push()
	e.storeTracesTagValues(map[string]tagValueCountFromDB{
		"aws.request_id": {tagDataType: "string", stringTagValueCount: 10},
	})
	push()
	e.storeTracesTagValues(map[string]tagValueCountFromDB{})
	push()

	require.Len(t, conn.rows, 1, "the same set is written once")
	assert.Empty(t, attrValues(conn.rows, "aws.request_id"), "a key found over the limit stays out when later counts are under it")
	assert.Equal(t, []string{"/users"}, attrValues(conn.rows, "http.route"))
}

func TestPushMetricsFiltersAttributesAndUsesTheMetricsBucket(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Metrics.MaxStringLength = 16
		cfg.MaxDistinctValues.Metrics.MaxResourceStringLength = 16
	})

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", "node-1")
	rm.Resource().Attributes().PutStr("os.description", strings.Repeat("Linux 6.1 ", 8))
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("system.disk.io")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("device", "disk0")
	dp.Attributes().PutStr("mount.options", strings.Repeat("rw,", 20))
	require.NoError(t, e.PushMetrics(context.Background(), md))

	require.Len(t, conn.rows, 1)
	row := conn.rows[0]
	assert.Equal(t, "metrics", row.signal)
	assert.Equal(t, map[string]string{"device": "disk0"}, row.attrs)
	assert.Equal(t, map[string]string{"host.name": "node-1"}, row.resourceAttrs)
	assert.Zero(t, row.unixMilli%DefaultMetricsBucket.Milliseconds(), "metrics rows are written per metrics bucket")
}

func TestPushLogsUsesTheObservedTimestampWhenTheTimestampIsUnset(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Logs.MaxStringLength = 16
	})
	bucket := e.cfg.MaxDistinctValues.Logs.Bucket
	observed := time.Now().Add(-3 * bucket)
	observedBucket := observed.UnixMilli() / bucket.Milliseconds() * bucket.Milliseconds()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(observed))
	lr.Attributes().PutStr("log.level", "error")
	lr.Attributes().PutStr("stacktrace", strings.Repeat("at frame\n", 10))
	require.NoError(t, e.PushLogs(context.Background(), ld))

	require.Len(t, conn.rows, 1)
	assert.Equal(t, "logs", conn.rows[0].signal)
	assert.Equal(t, observedBucket, conn.rows[0].unixMilli)
	assert.Equal(t, map[string]string{"log.level": "error"}, conn.rows[0].attrs)
}

func TestConfigValidateRejectsAZeroBucket(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())

	cfg.MaxDistinctValues.Metrics.Bucket = 0
	assert.ErrorContains(t, cfg.Validate(), "metrics::bucket")
}

func TestPushTracesWritesIntrinsicFields(t *testing.T) {
	e, conn := newTestExporter(t, nil)

	td := ptrace.NewTraces()
	span := newSpan(t, td, "POST /charge", time.Now(), map[string]any{
		"http.method":      "POST",
		"http.url":         "https://payments.example.com/charge",
		"http.status_code": int64(502),
		"db.name":          "orders",
	})
	span.SetKind(ptrace.SpanKindClient)
	span.Status().SetCode(ptrace.StatusCodeError)
	require.NoError(t, e.PushTraces(context.Background(), td))

	require.Len(t, conn.rows, 1)
	assert.Equal(t, map[string]string{
		"name":                 "POST /charge",
		"kind_string":          "Client",
		"status_code_string":   "Error",
		"has_error":            "true",
		"is_remote":            "unknown",
		"http_method":          "POST",
		"http_host":            "payments.example.com",
		"http_url":             "https://payments.example.com/charge",
		"response_status_code": "502",
		"db_name":              "orders",
		"external_http_method": "POST",
		"external_http_url":    "payments.example.com",
	}, conn.rows[0].intrinsics)
	assert.Equal(t, map[string]string{
		"name":        "POST /charge",
		"http.method": "POST",
		"http.url":    "https://payments.example.com/charge",
		"db.name":     "orders",
	}, conn.rows[0].attrs, "the span name is still written to attributes; calculated fields are not")
}

func TestPushTracesIntrinsicFieldsTellSetsApart(t *testing.T) {
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxStringLength = 16
	})
	now := time.Now()

	td := ptrace.NewTraces()
	newSpan(t, td, "GET /users", now, map[string]any{"http.route": "/users"})
	newSpan(t, td, "GET /users", now, map[string]any{"http.route": "/users"}).Status().SetCode(ptrace.StatusCodeError)
	newSpan(t, td, strings.Repeat("n", 32), now, map[string]any{"http.route": "/users"})
	newSpan(t, td, "GET /orders", now, map[string]any{"http.route": "/users"})
	require.NoError(t, e.PushTraces(context.Background(), td))

	require.Len(t, conn.rows, 4, "same attributes with different intrinsic fields are different sets")
	names := []string{}
	for _, row := range conn.rows {
		names = append(names, row.intrinsics["name"])
	}
	assert.ElementsMatch(t, []string{"GET /users", "GET /users", "", "GET /orders"}, names, "an oversized name is dropped from its row without taking the field out of later rows")
}

func TestPushLogsWritesSeverityFields(t *testing.T) {
	e, conn := newTestExporter(t, nil)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.SetSeverityText("ERROR")
	lr.SetSeverityNumber(plog.SeverityNumberError)
	lr.Attributes().PutStr("logger", "payments")
	require.NoError(t, e.PushLogs(context.Background(), ld))

	require.Len(t, conn.rows, 1)
	assert.Equal(t, map[string]string{"severity_text": "ERROR", "severity_number": "17"}, conn.rows[0].intrinsics)
	assert.Equal(t, map[string]string{"logger": "payments"}, conn.rows[0].attrs)
}

func TestPushLogsTellsApartSetsWhoseAttributesEqualTheirIntrinsics(t *testing.T) {
	e, conn := newTestExporter(t, nil)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	for _, severity := range []string{"INFO", "ERROR"} {
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		lr.SetSeverityText(severity)
		lr.Attributes().PutStr("severity_text", severity)
	}
	require.NoError(t, e.PushLogs(context.Background(), ld))

	require.Len(t, conn.rows, 2, "sets whose attribute map equals their intrinsic map must not share a fingerprint")
}

func TestConfigValidateRejectsASubMillisecondBucket(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxDistinctValues.Logs.Bucket = time.Microsecond
	assert.ErrorContains(t, cfg.Validate(), "logs::bucket")
}

func BenchmarkPushTracesRepeatedValues(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.Enabled = true
	conn := &fakeConn{}
	e, err := newMetadataExporterWithConn(context.Background(), *cfg, exportertest.NewNopSettings(metadata.Type), conn)
	require.NoError(b, err)
	defer func() { _ = e.Shutdown(context.Background()) }()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	rs.Resource().Attributes().PutStr("deployment.environment", "prod")
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	now := time.Now()
	for i := 0; i < 1000; i++ {
		span := spans.AppendEmpty()
		span.SetName("GET /users/{id}")
		span.SetKind(ptrace.SpanKindServer)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
		attrs := span.Attributes()
		attrs.PutStr("http.method", "GET")
		attrs.PutStr("http.route", "/users/{id}")
		attrs.PutStr("http.status_code", "200")
		attrs.PutStr("db.system", "postgresql")
		attrs.PutStr("db.operation", "SELECT")
		attrs.PutStr("rpc.service", "users")
		attrs.PutStr("user.tier", "free")
		attrs.PutStr("region", "us-east-1")
		attrs.PutStr("feature.flag", "flag-"+strconv.Itoa(i%20))
		attrs.PutStr("queue.name", "q-"+strconv.Itoa(i%5))
	}
	require.NoError(b, e.PushTraces(context.Background(), td))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.PushTraces(context.Background(), td)
	}
}

func TestPushTracesKeepsAnAlwaysIncludedIntrinsicFieldWhateverItsLength(t *testing.T) {
	name := strings.Repeat("GET /very/long/route/", 5)
	e, conn := newTestExporter(t, func(cfg *Config) {
		cfg.MaxDistinctValues.Traces.MaxStringLength = 16
		cfg.AlwaysIncludeAttributes.Traces = []string{"name"}
	})

	td := ptrace.NewTraces()
	newSpan(t, td, name, time.Now(), nil)
	require.NoError(t, e.PushTraces(context.Background(), td))

	require.Len(t, conn.rows, 1)
	assert.Equal(t, name, conn.rows[0].intrinsics["name"])
	assert.Equal(t, name, conn.rows[0].attrs["name"])
}
