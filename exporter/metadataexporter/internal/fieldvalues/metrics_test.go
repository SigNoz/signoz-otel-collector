package fieldvalues

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

func testMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.pod.name", "p1")
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otel-go")
	sm.Scope().SetVersion("1.0")
	sm.Scope().Attributes().PutStr("library.lang", "go")

	sum := sm.Metrics().AppendEmpty()
	sum.SetName("http_requests")
	sum.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for _, code := range []string{"200", "500"} {
		dp := sum.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(at("10:00"))
		dp.Attributes().PutStr("code", code)
		dp.Attributes().PutStr("method", "GET")
	}

	hist := sm.Metrics().AppendEmpty()
	hist.SetName("latency")
	hist.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	hdp := hist.Histogram().DataPoints().AppendEmpty()
	hdp.SetTimestamp(at("10:00"))
	hdp.Attributes().PutStr("route", "/cart")
	hdp.ExplicitBounds().FromRaw([]float64{0.1, 1})
	hdp.BucketCounts().FromRaw([]uint64{1, 2, 3})
	return md
}

// expectedSeriesID repeats the chain of the metrics exporter
// (signozclickhousemetrics), so the set id equals time_series_v4.fingerprint.
func expectedSeriesID(md pmetric.Metrics, metricIdx, dpIdx int, name string, temporality pmetric.AggregationTemporality) uint64 {
	rm := md.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	resourceFP := fingerprint.NewFingerprint(fingerprint.ResourceFingerprintType, fingerprint.InitialOffset, rm.Resource().Attributes(), map[string]string{})
	scopeFP := fingerprint.NewFingerprint(fingerprint.ScopeFingerprintType, resourceFP.Hash(), sm.Scope().Attributes(), map[string]string{
		"__scope.name__":       sm.Scope().Name(),
		"__scope.version__":    sm.Scope().Version(),
		"__scope.schema_url__": sm.SchemaUrl(),
	})
	m := sm.Metrics().At(metricIdx)
	var attrs pcommon.Map
	if m.Type() == pmetric.MetricTypeHistogram {
		attrs = m.Histogram().DataPoints().At(dpIdx).Attributes()
	} else {
		attrs = m.Sum().DataPoints().At(dpIdx).Attributes()
	}
	pointFP := fingerprint.NewFingerprint(fingerprint.PointFingerprintType, scopeFP.Hash(), attrs, map[string]string{
		"__temporality__": temporality.String(),
	})
	return pointFP.HashWithName(name)
}

func TestMetricsSetIsTheSeries(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	md := testMetrics()
	require.NoError(t, e.WriteMetrics(context.Background(), md))
	rows := w.take()

	bySeries := map[uint64]string{}
	for _, r := range rows {
		if r.attrsHash != resourceAttrsHash {
			bySeries[r.attrsHash] = r.metricName
		}
	}
	assert.Equal(t, map[uint64]string{
		expectedSeriesID(md, 0, 0, "http_requests", pmetric.AggregationTemporalityCumulative): "http_requests",
		expectedSeriesID(md, 0, 1, "http_requests", pmetric.AggregationTemporalityCumulative): "http_requests",
		expectedSeriesID(md, 1, 0, "latency.count", pmetric.AggregationTemporalityDelta):      "latency.count",
	}, bySeries, "one set per series, with the time_series_v4 fingerprint; a histogram writes only its .count series")

	assert.Equal(t, []string{
		"{code=200, library.lang=go, method=GET}",
		"{code=500, library.lang=go, method=GET}",
		"{library.lang=go, route=/cart}",
	}, setList(rows), "point and scope labels; no __temporality__, __scope.*__ or le rows")

	assert.Equal(t, []string{
		"__name__=http_requests*", "__name__=latency.count*",
		"http_requests:k8s.pod.name=*", "http_requests:service.name=*",
		"k8s.pod.name=p1",
		"latency.count:k8s.pod.name=*", "latency.count:service.name=*",
		"service.name=checkout",
	}, resourceRows(rows), "the resource once with no metric, a link row per metric, and a key row per metric and resource field")

	require.NoError(t, e.WriteMetrics(context.Background(), testMetrics()))
	assert.Empty(t, w.take())
}

func TestMetricsKeysSurviveAFullCache(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	// Room for metric a: its series, its label code, the resource and the link
	// of a. Nothing more fits in the exact part.
	e.state.cache.limit = [2]int{4, 10}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	sm := rm.ScopeMetrics().AppendEmpty()
	addSum := func(name string, labels map[string]any) {
		m := sm.Metrics().AppendEmpty()
		m.SetName(name)
		m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := m.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(at("10:00"))
		require.NoError(t, dp.Attributes().FromRaw(labels))
	}
	addSum("a", map[string]any{"code": "200"})
	addSum("b", map[string]any{"code": "200", "region": "eu"})
	addSum("b", map[string]any{"code": "500", "region": "us"})
	require.NoError(t, e.WriteMetrics(context.Background(), md))

	keys := map[string][]string{}
	for _, r := range w.take() {
		keys[r.metricName] = append(keys[r.metricName], r.p.ctx.String()+":"+r.p.name)
	}
	assert.ElementsMatch(t, []string{"attribute:code", "resource:service.name"}, keys["a"])
	assert.ElementsMatch(t, []string{"attribute:code", "attribute:region", "resource:service.name"}, keys["b"],
		"the series of b do not fit, but each label of b still gets one row, so its keys are complete")
}

// A resource of many metrics writes its rows once. Each metric adds one link
// row, and key rows only for fields it has no row for in the window.
func TestMetricsResourceRowsOnceAcrossMetrics(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	ctx := context.Background()
	pod := func(name string, metrics ...string) pmetric.Metrics {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("k8s.pod.name", name)
		rm.Resource().Attributes().PutStr("k8s.namespace.name", "shop")
		sm := rm.ScopeMetrics().AppendEmpty()
		for _, m := range metrics {
			g := sm.Metrics().AppendEmpty()
			g.SetName(m)
			g.SetEmptyGauge().DataPoints().AppendEmpty().SetTimestamp(at("10:00"))
		}
		return md
	}

	require.NoError(t, e.WriteMetrics(ctx, pod("p1", "cpu", "memory", "disk")))
	assert.Equal(t, []string{
		"__name__=cpu*", "__name__=disk*", "__name__=memory*",
		"cpu:k8s.namespace.name=*", "cpu:k8s.pod.name=*",
		"disk:k8s.namespace.name=*", "disk:k8s.pod.name=*",
		"k8s.namespace.name=shop", "k8s.pod.name=p1",
		"memory:k8s.namespace.name=*", "memory:k8s.pod.name=*",
	}, resourceRows(w.take()))

	require.NoError(t, e.WriteMetrics(ctx, pod("p2", "cpu", "memory")))
	assert.Equal(t, []string{
		"__name__=cpu*", "__name__=memory*",
		"k8s.namespace.name=shop", "k8s.pod.name=p2",
	}, resourceRows(w.take()), "a new pod writes its rows and links; the keys of its metrics are known")

	require.NoError(t, e.WriteMetrics(ctx, pod("p2", "cpu", "memory")))
	assert.Empty(t, w.take())
}
