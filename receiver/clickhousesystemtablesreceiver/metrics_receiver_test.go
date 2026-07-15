package clickhousesystemtablesreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver/internal/metadata"
)

func mustObsReport(t *testing.T) *receiverhelper.ObsReport {
	t.Helper()
	o, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "clickhouse",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	require.NoError(t, err)
	return o
}

func newTestMetricsReceiver(t *testing.T, ch clickhouseQuerier, cfg SystemTablesMetricsConfig) (*systemTablesMetricsReceiver, *consumertest.MetricsSink) {
	t.Helper()
	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = defaultMetricsCollectionInterval
	}
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(metadata.Type)
	return &systemTablesMetricsReceiver{
		config:       &Config{},
		metricsCfg:   &cfg,
		mb:           metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		clickhouse:   ch,
		nextConsumer: sink,
		logger:       zap.NewNop(),
		obsrecv:      mustObsReport(t),
	}, sink
}

func defaultMetricsScrapeConfig() SystemTablesMetricsConfig {
	return SystemTablesMetricsConfig{
		CollectionInterval:   defaultMetricsCollectionInterval,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func TestMetricsReceiverStartShutdown(t *testing.T) {
	r, _ := newTestMetricsReceiver(t, &mockClickhouseQuerrier{}, defaultMetricsScrapeConfig())
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestMetricsScrapeGroupsByHostname(t *testing.T) {
	require := require.New(t)

	mock := &mockClickhouseQuerrier{
		viewRefreshes: []ViewRefreshRow{
			{Hostname: "host-1", Database: "signoz_metrics", View: "samples_reduced_mv", LastSuccessAge: 12, LastDuration: 0.5, Exception: 0, Retry: 0, Progress: 1},
			{Hostname: "host-2", Database: "signoz_metrics", View: "samples_reduced_mv", LastSuccessAge: 4000, LastDuration: 2, Exception: 1, Retry: 3, Progress: 0},
		},
	}

	r, _ := newTestMetricsReceiver(t, mock, defaultMetricsScrapeConfig())
	md, err := r.collect(context.Background())
	require.NoError(err)

	byHost := metricsByHostname(md)
	require.ElementsMatch([]string{"host-1", "host-2"}, keys(byHost))

	// Each replica's view_refresh row is recorded under its own hostname resource.
	require.EqualValues(1, gaugeIntValue(t, byHost["host-2"], "clickhouse.view_refresh.exception"))
	require.EqualValues(0, gaugeIntValue(t, byHost["host-1"], "clickhouse.view_refresh.exception"))
	require.EqualValues(4000, gaugeIntValue(t, byHost["host-2"], "clickhouse.view_refresh.last_success_age"))
	require.EqualValues(3, gaugeIntValue(t, byHost["host-2"], "clickhouse.view_refresh.retry"))
	require.True(hasMetric(byHost["host-1"], "clickhouse.view_refresh.last_duration"))
}

func TestMetricsSkipsNeverSucceededAge(t *testing.T) {
	require := require.New(t)

	mock := &mockClickhouseQuerrier{
		viewRefreshes: []ViewRefreshRow{
			{Hostname: "host-1", Database: "signoz_metrics", View: "mv", LastSuccessAge: -1, LastDuration: 0, Exception: 0, Retry: 0, Progress: 0},
		},
	}
	r, _ := newTestMetricsReceiver(t, mock, defaultMetricsScrapeConfig())
	md, err := r.collect(context.Background())
	require.NoError(err)

	byHost := metricsByHostname(md)
	require.False(hasMetric(byHost["host-1"], "clickhouse.view_refresh.last_success_age"))
	// other metrics still recorded
	require.True(hasMetric(byHost["host-1"], "clickhouse.view_refresh.exception"))
}

func TestMetricsScrapeError(t *testing.T) {
	require := require.New(t)

	// A scrape failure surfaces as an error (reported to obsreport) with no metrics.
	// The run loop logs it and continues on the next tick.
	mock := &mockClickhouseQuerrier{viewRefreshErr: context.DeadlineExceeded}
	r, _ := newTestMetricsReceiver(t, mock, defaultMetricsScrapeConfig())
	md, err := r.collect(context.Background())
	require.Error(err)
	require.Equal(0, md.DataPointCount())
}

// --- helpers ---

func metricsByHostname(md pmetric.Metrics) map[string]pmetric.MetricSlice {
	out := map[string]pmetric.MetricSlice{}
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		hn, ok := rm.Resource().Attributes().Get("clickhouse.hostname")
		if !ok {
			continue
		}
		out[hn.Str()] = rm.ScopeMetrics().At(0).Metrics()
	}
	return out
}

func keys(m map[string]pmetric.MetricSlice) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func hasMetric(ms pmetric.MetricSlice, name string) bool {
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Name() == name {
			return true
		}
	}
	return false
}

func gaugeIntValue(t *testing.T, ms pmetric.MetricSlice, name string) int64 {
	t.Helper()
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Name() == name {
			return ms.At(i).Gauge().DataPoints().At(0).IntValue()
		}
	}
	t.Fatalf("metric %s not found", name)
	return 0
}
