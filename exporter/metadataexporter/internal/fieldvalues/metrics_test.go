package fieldvalues

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
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

func TestMetricsSetIsTheSeries(t *testing.T) {
	e, w := newTestExporter(t, testConfig(), pipeline.SignalMetrics)
	require.NoError(t, e.WriteMetrics(context.Background(), testMetrics()))
	rows := w.take()

	bySeries := map[uint64]string{}
	for _, r := range rows {
		if r.attrsHash != resourceAttrsHash {
			bySeries[r.attrsHash] = r.metricName
		}
	}
	assert.ElementsMatch(t, []string{"http_requests", "http_requests", "latency.count"}, slices.Collect(maps.Values(bySeries)),
		"one set per series; a histogram writes only its .count series")

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

// A label with a value per point makes each point a new series. Past the
// limit, the label leaves the set: the series of a route become one set, the
// label keeps a daily sample, and the series are not cached.
func TestMetricsLabelOverTheLimitLeavesTheSet(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxRecordFieldValues = 3
	e, w := newTestExporter(t, cfg, pipeline.SignalMetrics)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("requests")
	points := m.SetEmptySum().DataPoints()
	for i := 1; i <= 10; i++ {
		dp := points.AppendEmpty()
		dp.SetTimestamp(at("10:00"))
		dp.Attributes().PutStr("route", "/cart")
		dp.Attributes().PutStr("request_id", fmt.Sprintf("r%02d", i))
	}
	require.NoError(t, e.WriteMetrics(context.Background(), md))

	assert.Equal(t, []string{
		"{request_id=r01, route=/cart}",
		"{request_id=r02, route=/cart}",
		"{request_id=r03, route=/cart}",
		"{request_id=r04*, route=/cart}",
	}, setList(w.take()), "3 series in full; r04 passes the limit into the set of /cart; r05 on are past the sample budget")
	assert.Equal(t, 9, e.state.cache.used[classExact],
		"3 series, 1 coarse set, the resource, its link, its key row, and the keys of route and request_id; the series past the limit are not cached")

	require.NoError(t, e.WriteMetrics(context.Background(), md))
	assert.Empty(t, w.take())
}

// processMetrics is the output of the process scraper on one host: one
// resource per process, with two series each.
func processMetrics(processes int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for pid := 1; pid <= processes; pid++ {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("host.name", "node-7")
		rm.Resource().Attributes().PutInt("process.pid", int64(pid))
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("process.cpu.time")
		points := m.SetEmptySum().DataPoints()
		for _, state := range []string{"user", "system"} {
			dp := points.AppendEmpty()
			dp.SetTimestamp(at("10:00"))
			dp.Attributes().PutStr("state", state)
		}
	}
	return md
}

// Past the limit, process.pid leaves the identity: the processes of a host
// are one resource with one set per series, and each pid is still written
// once.
func TestMetricsResourceFieldOverTheLimitLeavesTheIdentity(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxResourceFieldValues = 10
	e, w := newTestExporter(t, cfg, pipeline.SignalMetrics)
	type counts struct{ resources, series, pidsInHash, pidsOutside int }
	count := func(rows []row) counts {
		resources := map[uint64]struct{}{}
		series := map[[2]uint64]struct{}{}
		var c counts
		for _, r := range rows {
			resources[r.resourceHash] = struct{}{}
			switch {
			case r.attrsHash != resourceAttrsHash:
				series[[2]uint64{r.resourceHash, r.attrsHash}] = struct{}{}
			case r.metricName == "" && r.p.name == "process.pid" && r.inHash:
				c.pidsInHash++
			case r.metricName == "" && r.p.name == "process.pid":
				c.pidsOutside++
			}
		}
		c.resources, c.series = len(resources), len(series)
		return c
	}

	require.NoError(t, e.WriteMetrics(context.Background(), processMetrics(200)))
	assert.Equal(t, counts{resources: 11, series: 22, pidsInHash: 10, pidsOutside: 190}, count(w.take()),
		"10 processes in full, then one resource for the host: 2 series for each resource, and each pid once")

	require.NoError(t, e.WriteMetrics(context.Background(), processMetrics(200)))
	assert.Equal(t, counts{resources: 1, pidsOutside: 10}, count(w.take()),
		"the first 10 processes join the host, so their pids are written outside its identity")

	require.NoError(t, e.WriteMetrics(context.Background(), processMetrics(200)))
	assert.Empty(t, w.take())
}
