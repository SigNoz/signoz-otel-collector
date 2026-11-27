package metadataexporter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	kash "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/cache"
	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type fakeBatch struct {
	driver.Batch
	conn      *fakeConn
	appendErr error
	onSend    func() error
}

func (b *fakeBatch) Append(_ ...any) error {
	b.conn.mu.Lock()
	b.conn.appendCalls++
	b.conn.mu.Unlock()
	if b.appendErr != nil {
		return b.appendErr
	}
	return nil
}

func (b *fakeBatch) Send() error {
	b.conn.mu.Lock()
	b.conn.sendCalls++
	b.conn.mu.Unlock()
	if b.onSend != nil {
		return b.onSend()
	}
	return nil
}

func (b *fakeBatch) Close() error {
	return nil
}

type fakeConn struct {
	driver.Conn
	mu           sync.Mutex
	prepareBatch func(ctx context.Context, attempt int) (driver.Batch, error)
	deadlines    []time.Time
	prepareCalls int
	appendCalls  int
	sendCalls    int
}

func (c *fakeConn) PrepareBatch(ctx context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.mu.Lock()
	c.prepareCalls++
	attempt := c.prepareCalls
	if deadline, ok := ctx.Deadline(); ok {
		c.deadlines = append(c.deadlines, deadline)
	}
	prepare := c.prepareBatch
	c.mu.Unlock()

	if prepare != nil {
		return prepare(ctx, attempt)
	}
	return &fakeBatch{conn: c}, nil
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.appendCalls, c.sendCalls
}

type spyKeyCache struct {
	kash.KeyCache
	mu             sync.Mutex
	addCalls       int
	attrsExistErr  error
	cardinalityErr error
}

func (s *spyKeyCache) AddAttrsToResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) error {
	s.mu.Lock()
	s.addCalls++
	s.mu.Unlock()
	return s.KeyCache.AddAttrsToResource(ctx, resourceFp, attrFps, ds)
}

func (s *spyKeyCache) AttrsExistForResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) ([]bool, error) {
	s.mu.Lock()
	err := s.attrsExistErr
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.KeyCache.AttrsExistForResource(ctx, resourceFp, attrFps, ds)
}

func (s *spyKeyCache) CardinalityLimitExceededMulti(ctx context.Context, resourceFps []uint64, ds pipeline.Signal) ([]bool, error) {
	s.mu.Lock()
	err := s.cardinalityErr
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.KeyCache.CardinalityLimitExceededMulti(ctx, resourceFps, ds)
}

func (s *spyKeyCache) addCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addCalls
}

func makeTestMetadataConfig(timeout time.Duration) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Enabled = true
	cfg.TimeoutConfig = exporterhelper.TimeoutConfig{Timeout: timeout}
	cfg.BackOffConfig = configretry.BackOffConfig{Enabled: false}
	cfg.QueueBatchConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	return cfg
}

func buildTestTraces(serviceName, attrVal, spanName string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", serviceName)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(spanName)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1760000000, 0)))
	span.Attributes().PutStr("label", attrVal)
	return td
}

func buildTestMetrics(serviceName, attrVal, metricName string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", serviceName)
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(metricName)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("label", attrVal)
	dp.SetIntValue(1)
	return md
}

func buildTestLogs(serviceName, attrVal, body string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", serviceName)
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1760000000, 0)))
	lr.Body().SetStr(body)
	lr.Attributes().PutStr("label", attrVal)
	return ld
}

func TestMetadataExporterFactoryErrorsAndTimeout(t *testing.T) {
	testCases := []struct {
		name         string
		signal       pipeline.Signal
		serviceName  string
		attrVal      string
		extra        string
		timeout      time.Duration
		prepareErr   error
		failedMetric string
	}{
		{
			name:         "Traces_PrepareBatchError_PropagatesAndRecordsTelemetry",
			signal:       pipeline.SignalTraces,
			serviceName:  "svc-trace-err",
			attrVal:      "v-trace-err",
			extra:        "span-err",
			timeout:      5 * time.Second,
			prepareErr:   errors.New("traces prepare failed"),
			failedMetric: "otelcol_exporter_send_failed_spans",
		},
		{
			name:         "Metrics_PrepareBatchError_PropagatesAndRecordsTelemetry",
			signal:       pipeline.SignalMetrics,
			serviceName:  "svc-metric-err",
			attrVal:      "v-metric-err",
			extra:        "metric-err",
			timeout:      5 * time.Second,
			prepareErr:   errors.New("metrics prepare failed"),
			failedMetric: "otelcol_exporter_send_failed_metric_points",
		},
		{
			name:         "Logs_PrepareBatchError_PropagatesAndRecordsTelemetry",
			signal:       pipeline.SignalLogs,
			serviceName:  "svc-log-err",
			attrVal:      "v-log-err",
			extra:        "log-err",
			timeout:      5 * time.Second,
			prepareErr:   errors.New("logs prepare failed"),
			failedMetric: "otelcol_exporter_send_failed_log_records",
		},
		{
			name:        "Traces_ConfiguredTimeout_ReachesPrepareBatch",
			signal:      pipeline.SignalTraces,
			serviceName: "svc-trace-timeout",
			attrVal:     "v-trace-timeout",
			extra:       "span-timeout",
			timeout:     10 * time.Second,
		},
		{
			name:        "Metrics_ConfiguredTimeout_ReachesPrepareBatch",
			signal:      pipeline.SignalMetrics,
			serviceName: "svc-metric-timeout",
			attrVal:     "v-metric-timeout",
			extra:       "metric-timeout",
			timeout:     10 * time.Second,
		},
		{
			name:        "Logs_ConfiguredTimeout_ReachesPrepareBatch",
			signal:      pipeline.SignalLogs,
			serviceName: "svc-log-timeout",
			attrVal:     "v-log-timeout",
			extra:       "log-timeout",
			timeout:     10 * time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })

			set := exportertest.NewNopSettings(metadata.Type)
			set.TelemetrySettings = tel.NewTelemetrySettings()

			var conn *fakeConn
			conn = &fakeConn{
				prepareBatch: func(_ context.Context, _ int) (driver.Batch, error) {
					if testCase.prepareErr != nil {
						return nil, testCase.prepareErr
					}
					return &fakeBatch{conn: conn}, nil
				},
			}

			factory := &metadataExporterFactory{
				openConn: func(string) (driver.Conn, error) {
					return conn, nil
				},
			}
			cfg := makeTestMetadataConfig(testCase.timeout)

			var consumeErr error
			switch testCase.signal {
			case pipeline.SignalTraces:
				exp, err := factory.createTracesExporter(context.Background(), set, cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.Background())) })
				consumeErr = exp.ConsumeTraces(context.Background(), buildTestTraces(testCase.serviceName, testCase.attrVal, testCase.extra))
			case pipeline.SignalMetrics:
				exp, err := factory.createMetricsExporter(context.Background(), set, cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.Background())) })
				consumeErr = exp.ConsumeMetrics(context.Background(), buildTestMetrics(testCase.serviceName, testCase.attrVal, testCase.extra))
			case pipeline.SignalLogs:
				exp, err := factory.createLogsExporter(context.Background(), set, cfg)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.Background())) })
				consumeErr = exp.ConsumeLogs(context.Background(), buildTestLogs(testCase.serviceName, testCase.attrVal, testCase.extra))
			}

			if testCase.prepareErr != nil {
				require.ErrorIs(t, consumeErr, testCase.prepareErr)
				gotMetric, err := tel.GetMetric(testCase.failedMetric)
				require.NoError(t, err)
				sum, ok := gotMetric.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.Len(t, sum.DataPoints, 1)
				assert.Equal(t, int64(1), sum.DataPoints[0].Value)
				return
			}

			require.NoError(t, consumeErr)
			conn.mu.Lock()
			deadlines := append([]time.Time(nil), conn.deadlines...)
			conn.mu.Unlock()
			require.Len(t, deadlines, 1)
			remaining := time.Until(deadlines[0])
			assert.Greater(t, remaining, 5*time.Second)
			assert.LessOrEqual(t, remaining, testCase.timeout)
		})
	}
}

func TestMetadataExporterWriteErrorsAndCache(t *testing.T) {
	pushSignal := func(ctx context.Context, exp *metadataExporter, sig pipeline.Signal, serviceName, attrVal, extra string) error {
		switch sig {
		case pipeline.SignalTraces:
			return exp.PushTraces(ctx, buildTestTraces(serviceName, attrVal, extra))
		case pipeline.SignalMetrics:
			return exp.PushMetrics(ctx, buildTestMetrics(serviceName, attrVal, extra))
		case pipeline.SignalLogs:
			return exp.PushLogs(ctx, buildTestLogs(serviceName, attrVal, extra))
		default:
			return nil
		}
	}

	signalFingerprints := func(sig pipeline.Signal, serviceName, attrVal, extra string) (uint64, uint64) {
		rFp := fingerprint.FingerprintHash(map[string]any{"service.name": serviceName})
		attrs := map[string]any{"label": attrVal}
		if sig == pipeline.SignalTraces {
			attrs["name"] = extra
		}
		return rFp, fingerprint.FingerprintHash(attrs)
	}

	testCases := []struct {
		name        string
		signal      pipeline.Signal
		serviceName string
		attrVal     string
		extra       string
		mode        string
		err         error
	}{
		{
			name:        "Traces_SendTimeout_RetryReappendsAndCachesAfterSuccess",
			signal:      pipeline.SignalTraces,
			serviceName: "svc-trace-send",
			attrVal:     "val-trace-send",
			extra:       "span-send",
			mode:        "send_retry",
		},
		{
			name:        "Metrics_SendTimeout_RetryReappendsAndCachesAfterSuccess",
			signal:      pipeline.SignalMetrics,
			serviceName: "svc-metric-send",
			attrVal:     "val-metric-send",
			extra:       "metric-send",
			mode:        "send_retry",
		},
		{
			name:        "Logs_SendTimeout_RetryReappendsAndCachesAfterSuccess",
			signal:      pipeline.SignalLogs,
			serviceName: "svc-log-send",
			attrVal:     "val-log-send",
			extra:       "log-send",
			mode:        "send_retry",
		},
		{
			name:        "Traces_AppendError_PropagatesAndSkipsSendAndCache",
			signal:      pipeline.SignalTraces,
			serviceName: "svc-trace-append",
			attrVal:     "val-trace-append",
			extra:       "span-append",
			mode:        "append_err",
			err:         errors.New("trace append failed"),
		},
		{
			name:        "Metrics_AppendError_PropagatesAndSkipsSendAndCache",
			signal:      pipeline.SignalMetrics,
			serviceName: "svc-metric-append",
			attrVal:     "val-metric-append",
			extra:       "metric-append",
			mode:        "append_err",
			err:         errors.New("metric append failed"),
		},
		{
			name:        "Logs_AppendError_PropagatesAndSkipsSendAndCache",
			signal:      pipeline.SignalLogs,
			serviceName: "svc-log-append",
			attrVal:     "val-log-append",
			extra:       "log-append",
			mode:        "append_err",
			err:         errors.New("log append failed"),
		},
		{
			name:        "Traces_AttrsExistError_PropagatesError",
			signal:      pipeline.SignalTraces,
			serviceName: "svc-trace-lookup",
			attrVal:     "val-trace-lookup",
			extra:       "span-lookup",
			mode:        "attrs_exist_err",
			err:         errors.New("attrs lookup failed"),
		},
		{
			name:        "Metrics_CardinalityLookupError_PropagatesError",
			signal:      pipeline.SignalMetrics,
			serviceName: "svc-metric-card",
			attrVal:     "val-metric-card",
			extra:       "metric-card",
			mode:        "cardinality_err",
			err:         errors.New("cardinality lookup failed"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var conn *fakeConn
			var exp *metadataExporter
			var spy *spyKeyCache
			resourceFp, attrFp := signalFingerprints(testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)

			var cachedDuringFirstSend bool
			conn = &fakeConn{
				prepareBatch: func(_ context.Context, attempt int) (driver.Batch, error) {
					batch := &fakeBatch{conn: conn}
					switch testCase.mode {
					case "send_retry":
						batch.onSend = func() error {
							if attempt == 1 {
								exists, err := spy.KeyCache.AttrsExistForResource(context.Background(), resourceFp, []uint64{attrFp}, testCase.signal)
								require.NoError(t, err)
								cachedDuringFirstSend = exists[0]
								return context.DeadlineExceeded
							}
							return nil
						}
					case "append_err":
						batch.appendErr = testCase.err
					}
					return batch, nil
				},
			}

			var err error
			exp, err = newMetadataExporter(context.Background(), *makeTestMetadataConfig(5 * time.Second), exportertest.NewNopSettings(metadata.Type), conn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.Background())) })

			spy = &spyKeyCache{KeyCache: exp.keyCache}
			if testCase.mode == "attrs_exist_err" {
				spy.attrsExistErr = testCase.err
			} else if testCase.mode == "cardinality_err" {
				spy.cardinalityErr = testCase.err
			}
			exp.keyCache = spy

			ctx := context.Background()
			switch testCase.mode {
			case "send_retry":
				err = pushSignal(ctx, exp, testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.False(t, cachedDuringFirstSend)
				assert.Equal(t, 0, spy.addCount())

				exists, existErr := spy.KeyCache.AttrsExistForResource(ctx, resourceFp, []uint64{attrFp}, testCase.signal)
				require.NoError(t, existErr)
				assert.False(t, exists[0])

				err = pushSignal(ctx, exp, testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)
				require.NoError(t, err)
				appendsAfterRetry, sendsAfterRetry := conn.counts()
				assert.Equal(t, 2, appendsAfterRetry)
				assert.Equal(t, 2, sendsAfterRetry)
				assert.Equal(t, 1, spy.addCount())

				exists, existErr = spy.KeyCache.AttrsExistForResource(ctx, resourceFp, []uint64{attrFp}, testCase.signal)
				require.NoError(t, existErr)
				assert.True(t, exists[0])

				err = pushSignal(ctx, exp, testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)
				require.NoError(t, err)
				appendsAfterHit, _ := conn.counts()
				assert.Equal(t, 2, appendsAfterHit)

			case "append_err":
				err = pushSignal(ctx, exp, testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)
				require.ErrorIs(t, err, testCase.err)
				_, sendCalls := conn.counts()
				assert.Equal(t, 0, sendCalls)
				assert.Equal(t, 0, spy.addCount())

				exists, existErr := spy.KeyCache.AttrsExistForResource(ctx, resourceFp, []uint64{attrFp}, testCase.signal)
				require.NoError(t, existErr)
				assert.False(t, exists[0])

			case "attrs_exist_err", "cardinality_err":
				err = pushSignal(ctx, exp, testCase.signal, testCase.serviceName, testCase.attrVal, testCase.extra)
				require.ErrorIs(t, err, testCase.err)
				appendCalls, sendCalls := conn.counts()
				assert.Equal(t, 0, appendCalls)
				assert.Equal(t, 0, sendCalls)
				assert.Equal(t, 0, spy.addCount())
			}
		})
	}
}
