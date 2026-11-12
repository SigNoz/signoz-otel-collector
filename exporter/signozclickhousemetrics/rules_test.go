package signozclickhousemetrics

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/metric/metricdata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	v1 "github.com/SigNoz/signoz-otel-collector/pkg/metering/v1"
	"github.com/SigNoz/signoz-otel-collector/pkg/metricreductionrule"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/SigNoz/signoz-otel-collector/usage"
)

func reductionTestConfig() *Config {
	return &Config{
		MetadataWriteSampleRatio: 1.0,
		Reduction: ReductionConfig{
			BufferSamplesTable:    "distributed_samples_v4_buffer",
			BufferTimeSeriesTable: "distributed_time_series_v4_buffer",
		},
	}
}

// testSource is the signozmetricreductionrules extension with fixed rules.
type testSource struct {
	component.StartFunc
	component.ShutdownFunc
	enabled bool
	rules   metricreductionrule.RuleSet
}

func (s testSource) Enabled() bool {
	return s.enabled
}

func (s testSource) Current() metricreductionrule.RuleSet {
	return s.rules
}

var rulesID = component.MustNewID("signozmetricreductionrules")

// waitForCacheCleanup returns once the cache cleanup goroutine started by Start is running.
// ttlcache's Stop does nothing before that goroutine runs, so Shutdown right after Start
// would leave it running.
func waitForCacheCleanup(t *testing.T, exp *clickhouseMetricsExporter) {
	t.Helper()
	exp.cache.Set("probe", true, time.Millisecond)
	require.Eventually(t, func() bool { return exp.cache.Len() == 0 }, time.Second, time.Millisecond)
}

// testHost serves these extensions; componenttest.NewNopHost has none.
type testHost map[component.ID]component.Component

func (h testHost) GetExtensions() map[component.ID]component.Component {
	return h
}

func newReductionExporter(t *testing.T, rules metricreductionrule.RuleSet) *clickhouseMetricsExporter {
	t.Helper()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(reductionTestConfig()),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	exp.rules = testSource{enabled: true, rules: rules}
	exp.reductionEnabled = true
	return exp
}

func labelSet(keys ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return set
}

func Test_reductionKeepMode(t *testing.T) {
	// keep mode: only the listed label (plus protected labels) survives; every
	// other label, including scope dunders, is dropped.
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"system.memory.usage0": metricreductionrule.NewReductionRule(labelSet("gauge.attr_0"), true, 0),
	})
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)

	require.Equal(t, 1, len(batch.samples))
	s := batch.samples[0]
	assert.NotZero(t, s.reducedFingerprint)
	assert.NotEqual(t, s.fingerprint, s.reducedFingerprint)

	require.Equal(t, 2, len(batch.ts))
	reducedTs := batch.ts[1]
	require.True(t, reducedTs.isReduced)
	assert.Contains(t, reducedTs.attrs, "gauge.attr_0")
	assert.NotContains(t, reducedTs.resourceAttrs, "resource.attr_0")
	assert.NotContains(t, reducedTs.scopeAttrs, "scope.attr_0")

	// cross-check the reduced fingerprint against a manual keep-only chain
	keepDrop := func(k string) bool {
		switch k {
		case "gauge.attr_0", "__name__", "__temporality__", "deployment.environment", "le", "quantile":
			return false
		}
		return true
	}
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("resource.attr_0", "value0")
	resourceFp := pkgfingerprint.NewFingerprint(pkgfingerprint.ResourceFingerprintType, pkgfingerprint.InitialOffset, resourceAttrs, map[string]string{})

	scopeAttrs := pcommon.NewMap()
	scopeAttrs.PutStr("scope.attr_0", "value0")
	scopeFp := pkgfingerprint.NewFingerprint(pkgfingerprint.ScopeFingerprintType, resourceFp.Hash(), scopeAttrs, map[string]string{
		"__scope.name__":       "go.signoz.io/app/reader",
		"__scope.version__":    "1.0.0",
		"__scope.schema_url__": "scope.schema_url",
	})

	pointAttrs := pcommon.NewMap()
	pointAttrs.PutStr("gauge.attr_0", "1")
	pointFp := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFp.Hash(), pointAttrs, map[string]string{
		"__temporality__": pmetric.AggregationTemporalityUnspecified.String(),
	})

	reducedResource := resourceFp.Reduced(pkgfingerprint.InitialOffset, keepDrop)
	reducedScope := scopeFp.Reduced(reducedResource.Hash(), keepDrop)
	reducedPoint := pointFp.Reduced(reducedScope.Hash(), keepDrop)
	assert.Equal(t, s.reducedFingerprint, reducedPoint.HashWithName("system.memory.usage0"))
}

func Test_reductionGauge(t *testing.T) {
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"system.memory.usage0": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	})
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)

	require.Equal(t, 1, len(batch.samples))
	s := batch.samples[0]
	assert.NotZero(t, s.reducedFingerprint)
	assert.NotEqual(t, s.fingerprint, s.reducedFingerprint)

	// raw series row carries the reduced fingerprint; reduced series row is its own entry
	require.Equal(t, 2, len(batch.ts))
	rawTs, reducedTs := batch.ts[0], batch.ts[1]
	assert.False(t, rawTs.isReduced)
	assert.Equal(t, s.fingerprint, rawTs.fingerprint)
	assert.Equal(t, s.reducedFingerprint, rawTs.reducedFingerprint)
	assert.True(t, reducedTs.isReduced)
	assert.Equal(t, s.reducedFingerprint, reducedTs.fingerprint)
	assert.Equal(t, s.reducedFingerprint, reducedTs.reducedFingerprint)
	assert.NotContains(t, reducedTs.resourceAttrs, "resource.attr_0")
	assert.NotContains(t, reducedTs.labels, "resource.attr_0")
	assert.Contains(t, rawTs.resourceAttrs, "resource.attr_0")

	// cross-check the reduced fingerprint against a manually built chain
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("resource.attr_0", "value0")
	resourceFp := pkgfingerprint.NewFingerprint(pkgfingerprint.ResourceFingerprintType, pkgfingerprint.InitialOffset, resourceAttrs, map[string]string{})

	scopeAttrs := pcommon.NewMap()
	scopeAttrs.PutStr("scope.attr_0", "value0")
	scopeFp := pkgfingerprint.NewFingerprint(pkgfingerprint.ScopeFingerprintType, resourceFp.Hash(), scopeAttrs, map[string]string{
		"__scope.name__":       "go.signoz.io/app/reader",
		"__scope.version__":    "1.0.0",
		"__scope.schema_url__": "scope.schema_url",
	})

	pointAttrs := pcommon.NewMap()
	pointAttrs.PutStr("gauge.attr_0", "1")
	pointFp := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFp.Hash(), pointAttrs, map[string]string{
		"__temporality__": pmetric.AggregationTemporalityUnspecified.String(),
	})
	require.Equal(t, s.fingerprint, pointFp.HashWithName("system.memory.usage0"))

	drop := func(k string) bool { return k == "resource.attr_0" }
	reducedResource := resourceFp.Reduced(pkgfingerprint.InitialOffset, drop)
	reducedScope := scopeFp.Reduced(reducedResource.Hash(), drop)
	reducedPoint := pointFp.Reduced(reducedScope.Hash(), drop)
	assert.Equal(t, s.reducedFingerprint, reducedPoint.HashWithName("system.memory.usage0"))
}

func Test_reductionEffectiveFromFuture(t *testing.T) {
	// datapoints stamped at 1727286182000; a rule effective after that must not apply
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"system.memory.usage0": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 1727286182001),
	})
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)

	require.Equal(t, 1, len(batch.samples))
	assert.Zero(t, batch.samples[0].reducedFingerprint)
	require.Equal(t, 1, len(batch.ts))
	assert.False(t, batch.ts[0].isReduced)
	assert.Zero(t, batch.ts[0].reducedFingerprint)
}

func Test_reductionNoRuleOrDisabled(t *testing.T) {
	// no rule for the metric
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"some.other.metric": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	})
	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)
	require.Equal(t, 1, len(batch.samples))
	assert.Zero(t, batch.samples[0].reducedFingerprint)

	// reduction disabled ignores injected rules entirely
	disabled, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	rules := metricreductionrule.RuleSet{"system.memory.usage0": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0)}
	disabled.rules = testSource{enabled: false, rules: rules}
	batch = disabled.prepareBatch(context.Background(), metrics)
	require.Equal(t, 1, len(batch.samples))
	assert.Zero(t, batch.samples[0].reducedFingerprint)
	require.Equal(t, 1, len(batch.ts))
}

func Test_reductionCollapsesAcrossResources(t *testing.T) {
	buildMetrics := func(instances ...string) pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		for _, instance := range instances {
			rm := metrics.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutStr("service.name", "app")
			rm.Resource().Attributes().PutStr("service.instance.id", instance)
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName("lib")
			m := sm.Metrics().AppendEmpty()
			m.SetName("http.server.requests.count")
			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(1727286182000)))
			dp.SetIntValue(10)
			dp.Attributes().PutStr("status", "ok")
		}
		return metrics
	}

	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"http.server.requests.count": metricreductionrule.NewReductionRule(labelSet("service.instance.id"), false, 0),
	})
	batch := exp.prepareBatch(context.Background(), buildMetrics("instance-1", "instance-2"))

	require.Equal(t, 2, len(batch.samples))
	assert.NotEqual(t, batch.samples[0].fingerprint, batch.samples[1].fingerprint)
	assert.NotZero(t, batch.samples[0].reducedFingerprint)
	assert.Equal(t, batch.samples[0].reducedFingerprint, batch.samples[1].reducedFingerprint)
}

func Test_reductionHistogramFlattenedRules(t *testing.T) {
	// rules are keyed by the flattened metric name: only the listed derived
	// metrics (.bucket and .count here) are reduced; .sum/.min/.max pass through.
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"http.server.duration0.bucket": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
		"http.server.duration0.count":  metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	})
	metrics := pmetricsgen.GenerateHistogramMetrics(1, 1, 1, 1, 1, 20, 20, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)

	require.NotEmpty(t, batch.samples)
	for _, s := range batch.samples {
		if strings.HasSuffix(s.metricName, bucketSuffix) || strings.HasSuffix(s.metricName, countSuffix) {
			assert.NotZero(t, s.reducedFingerprint, "ruled metric %s should be reduced", s.metricName)
		} else {
			assert.Zero(t, s.reducedFingerprint, "unruled metric %s should pass through", s.metricName)
		}
	}

	var reducedBuckets, reducedCount, reducedOther int
	for _, ts := range batch.ts {
		if !ts.isReduced {
			continue
		}
		switch {
		case strings.HasSuffix(ts.metricName, bucketSuffix):
			reducedBuckets++
			assert.Contains(t, ts.attrs, "le", "le is a protected label and must survive reduction")
			assert.NotContains(t, ts.resourceAttrs, "resource.attr_0")
		case strings.HasSuffix(ts.metricName, countSuffix):
			reducedCount++
		default:
			reducedOther++
		}
	}
	// 20 explicit bounds + the +Inf bucket
	assert.Equal(t, 21, reducedBuckets)
	assert.Equal(t, 1, reducedCount)
	assert.Zero(t, reducedOther, "only ruled metrics emit reduced rows")
}

func Test_reductionSkipsExponentialHistogram(t *testing.T) {
	exp, err := NewClickHouseExporter(
		WithEnableExpHist(true),
		WithLogger(zap.NewNop()),
		WithConfig(reductionTestConfig()),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	rules := metricreductionrule.RuleSet{
		"http.server.duration1": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	}
	exp.rules = testSource{enabled: true, rules: rules}
	exp.reductionEnabled = true

	metrics := pmetricsgen.GenerateExponentialHistogramMetrics(2, 1, 1, 1, 1, 22, 0, 0)
	batch := exp.prepareBatch(context.Background(), metrics)
	require.NotEmpty(t, batch.samples)
	for _, s := range batch.samples {
		assert.Zero(t, s.reducedFingerprint, "exponential histograms are excluded from reduction (%s)", s.metricName)
	}
	for _, ts := range batch.ts {
		assert.False(t, ts.isReduced)
	}
}

func Test_reductionRoutedMatchesMeter(t *testing.T) {
	// points are stamped base+0s, base+1s, ...; NaN and no-recorded points restart at base+0s
	const base, afterFirstPoint = 1727286182000, 1727286183000
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"system.memory.usage0":         metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, afterFirstPoint),
		"system.cpu.time1":             metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, base),
		"http.server.duration0":        metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, base),
		"http.server.duration0.sum":    metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, base),
		"http.server.duration0.min":    metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, afterFirstPoint),
		"http.server.duration0.bucket": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, afterFirstPoint),
		"zk.duration0.count":           metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, base),
		"zk.duration0.quantile":        metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, afterFirstPoint),
	})

	metrics := pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{
		GaugeMetricsCount:                  2,
		GaugeDataPointCount:                5,
		GaugeNanValuesCount:                1,
		GaugeNoRecordCount:                 1,
		SumMetricsCount:                    2,
		SumDataPointCount:                  3,
		SumNanValuesCount:                  1,
		HistogramMetricsCount:              1,
		HistogramDataPointCount:            3,
		HistogramExplicitBoundsCount:       2,
		HistogramBucketCount:               3,
		HistogramNanValuesCount:            1,
		ExponentialHistogramMetricsCount:   1,
		ExponentialHistogramDataPointCount: 2,
		ExponentialHistogramBucketCount:    4,
		SummaryMetricsCount:                1,
		SummaryDataPointCount:              3,
		SummaryQuantileCount:               2,
		SummaryNanValuesCount:              1,
	}))
	// gauges 0-1, sums 2-3, histogram 4, exponential histogram 5, summary 6
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	scopeMetrics.At(1).SetName("otelcol.exporter.sent_metric_points")
	histogramPoint := scopeMetrics.At(4).Histogram().DataPoints().At(1)
	histogramPoint.RemoveSum()
	histogramPoint.RemoveMin()
	pmetricsgen.Generate(
		pmetricsgen.WithResourceAttributeStringValue("other"),
		pmetricsgen.WithCount(pmetricsgen.Count{GaugeMetricsCount: 1, GaugeDataPointCount: 2}),
	).ResourceMetrics().MoveAndAppendTo(metrics.ResourceMetrics())

	batch := exp.prepareBatch(context.Background(), metrics)

	var samples, reduced int
	for _, sample := range batch.samples {
		if strings.HasPrefix(sample.metricName, "signoz") || strings.HasPrefix(sample.metricName, "otelcol") {
			continue
		}
		samples++
		if sample.reducedFingerprint != 0 {
			reduced++
		}
	}
	meter := v1.NewMetrics(nil)
	var count, routed int
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		count += meter.CountPerResource(metrics.ResourceMetrics().At(i))
		routed += meter.CountRoutedPerResource(metrics.ResourceMetrics().At(i), exp.rules.Current())
	}

	// first resource:
	//   system.memory.usage0: 3 valid (+0s..+2s) + 1 no-recorded (+0s) = 4, routed from +1s = 2
	//   otelcol.exporter.sent_metric_points: excluded by the meter
	//   system.cpu.time0: 2 valid = 2, no rule
	//   system.cpu.time1: 2 valid = 2, routed = 2
	//   http.server.duration0 point +0s: 2 bounds + le=+Inf (1) + count (1) + sum (1) + min (1) + max (1) = 7,
	//     routed sum (1) = 1
	//   http.server.duration0 point +1s without sum and min: 3 buckets + count (1) + max (1) = 5,
	//     routed buckets (3) = 3
	//   exponential histogram: not a sample, not counted
	//   zk.duration0: 2 points × (2 quantiles + count (1) + sum (1)) = 8, routed count (2) + quantiles at +1s (2) = 4
	// second resource: system.memory.usage0: 2 valid = 2, routed from +1s = 1
	// samples: 4 + 2 + 2 + 7 + 5 + 8 + 2 = 30; routed: 2 + 2 + 1 + 3 + 4 + 1 = 13
	assert.Equal(t, 30, count)
	assert.Equal(t, 13, routed)
	assert.Equal(t, count, samples)
	assert.Equal(t, routed, reduced)
}

func Test_collectUsageForSample(t *testing.T) {
	exp := newReductionExporter(t, metricreductionrule.RuleSet{
		"http.server.duration0.bucket": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	})

	assert.True(t, exp.collectUsageForSample(&sample{metricName: "http.server.duration0.count"}))
	assert.True(t, exp.collectUsageForSample(&sample{metricName: "chi.foo"}))

	// internal metrics
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "signoz.collector.foo"}))
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "otelcol.foo"}))

	// reduced
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "anything", reducedFingerprint: 42}))

	// ruled metric is skipped
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "http.server.duration0.bucket"}))

	disabledExp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	disabledExp.rules = testSource{enabled: false, rules: metricreductionrule.RuleSet{
		"http.server.duration0.bucket": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0),
	}}
	assert.True(t, disabledExp.collectUsageForSample(&sample{metricName: "http.server.duration0.bucket"}))
}

func Test_writeBatchReductionEnabled(t *testing.T) {
	conn, err := cmock.NewClickHouseNative(nil)
	require.NoError(t, err)
	conn.MatchExpectationsInOrder(false)
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, reduced_fingerprint, is_monotonic, unix_milli, value, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, reduced_fingerprint, is_reduced, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectPrepareBatch("INSERT INTO . (env, temporality, metric_name, fingerprint, unix_milli, count, sum, min, max, sketch, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectPrepareBatch("INSERT INTO . (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	conn.ExpectClose()

	cfg := reductionTestConfig()
	cfg.Extension = &rulesID
	cfg.Database = ""
	cfg.Reduction.BufferSamplesTable = ""
	cfg.Reduction.BufferTimeSeriesTable = ""
	exp, err := NewClickHouseExporter(
		WithConn(conn),
		WithLogger(zaptest.NewLogger(t)),
		WithConfig(cfg),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	rules := metricreductionrule.RuleSet{"system.memory.usage0": metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0)}
	require.NoError(t, exp.Start(context.Background(), testHost{rulesID: testSource{enabled: true, rules: rules}}))
	waitForCacheCleanup(t, exp)

	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	require.NoError(t, exp.PushMetrics(context.Background(), metrics))
	require.NoError(t, exp.Shutdown(context.Background()))
}

func Test_StartReduction(t *testing.T) {
	notSource := struct {
		component.StartFunc
		component.ShutdownFunc
	}{}

	testCases := []struct {
		name               string
		extension          *component.ID
		extensions         testHost
		expectedSamples    string
		expectedTimeSeries string
		// reduced usage reporter's first report: 3 lookback windows × 1 reduced table
		expectedReducedUsageQueries int
		expectedErr                 string
	}{
		{
			name:               "NoExtensionNamed_RawTables",
			extensions:         testHost{rulesID: testSource{enabled: true}},
			expectedSamples:    "INSERT INTO signoz_metrics.distributed_samples_v4 (",
			expectedTimeSeries: "INSERT INTO signoz_metrics.distributed_time_series_v4 (",
		},
		{
			name:               "ExtensionDisabled_RawTables",
			extension:          &rulesID,
			extensions:         testHost{rulesID: testSource{enabled: false}},
			expectedSamples:    "INSERT INTO signoz_metrics.distributed_samples_v4 (",
			expectedTimeSeries: "INSERT INTO signoz_metrics.distributed_time_series_v4 (",
		},
		{
			name:                        "ExtensionEnabled_BufferTables",
			extension:                   &rulesID,
			extensions:                  testHost{rulesID: testSource{enabled: true}},
			expectedSamples:             "INSERT INTO signoz_metrics.distributed_samples_v4_buffer (",
			expectedTimeSeries:          "INSERT INTO signoz_metrics.distributed_time_series_v4_buffer (",
			expectedReducedUsageQueries: 3,
		},
		{
			name:        "ExtensionNamedButMissing_Error",
			extension:   &rulesID,
			extensions:  testHost{},
			expectedErr: `metric reduction rules extension "signozmetricreductionrules" not found`,
		},
		{
			name:        "ExtensionNotSource_Error",
			extension:   &rulesID,
			extensions:  testHost{rulesID: notSource},
			expectedErr: `extension "signozmetricreductionrules" is not a metric reduction rules extension`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conn, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
			require.NoError(t, err)
			for range testCase.expectedReducedUsageQueries {
				conn.ExpectQueryRow(`distributed_samples_v4_reduced_last_60s`).
					WillReturnRow(cmock.NewRow([]cmock.ColumnType{{Name: "count()", Type: "UInt64"}}, []any{uint64(0)})).
					WillReturnError(errors.New("reduced usage query failed"))
			}
			conn.ExpectClose()
			usageCollector := usage.NewUsageCollector(
				uuid.New(),
				conn,
				usage.Options{
					ReportingInterval:  5 * time.Minute,
					ReducedUsageTables: []string{"distributed_samples_v4_reduced_last_60s"},
				},
				"signoz_metrics",
				func([]*metricdata.Metric, uuid.UUID) (map[string]usage.Usage, error) { return nil, nil },
				zap.NewNop(),
			)
			cfg := createDefaultConfig().(*Config)
			cfg.Extension = testCase.extension
			exp, err := NewClickHouseExporter(
				WithConn(conn),
				WithUsageCollector(usageCollector),
				WithLogger(zap.NewNop()),
				WithConfig(cfg),
				WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
			)
			require.NoError(t, err)

			err = exp.Start(context.Background(), testCase.extensions)
			if err == nil {
				waitForCacheCleanup(t, exp)
			}
			require.NoError(t, exp.Shutdown(context.Background()))

			if testCase.expectedErr != "" {
				assert.EqualError(t, err, testCase.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(exp.samplesSQL, testCase.expectedSamples), exp.samplesSQL)
			assert.True(t, strings.HasPrefix(exp.timeSeriesSQL, testCase.expectedTimeSeries), exp.timeSeriesSQL)
			assert.NoError(t, conn.ExpectationsWereMet())
		})
	}
}

func Benchmark_prepareBatchGaugeWithReduction(b *testing.B) {
	// same shape as Benchmark_prepareBatchGauge, every metric ruled
	metrics := pmetricsgen.GenerateGaugeMetrics(10000, 10, 10, 10, 10, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(reductionTestConfig()),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	if err != nil {
		b.Fatal(err)
	}
	rules := make(metricreductionrule.RuleSet, 10000)
	for i := 0; i < 10000; i++ {
		rules["system.memory.usage"+strconv.Itoa(i)] = metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0)
	}
	exp.rules = testSource{enabled: true, rules: rules}
	exp.reductionEnabled = true
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}

func Benchmark_prepareBatchGaugeWithReduction50k(b *testing.B) {
	// 5x the scale of Benchmark_prepareBatchGaugeWithReduction, every metric ruled
	const numMetrics = 50000
	metrics := pmetricsgen.GenerateGaugeMetrics(numMetrics, 10, 10, 10, 10, 0, 0)
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(reductionTestConfig()),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	if err != nil {
		b.Fatal(err)
	}
	rules := make(metricreductionrule.RuleSet, numMetrics)
	for i := 0; i < numMetrics; i++ {
		rules["system.memory.usage"+strconv.Itoa(i)] = metricreductionrule.NewReductionRule(labelSet("resource.attr_0"), false, 0)
	}
	exp.rules = testSource{enabled: true, rules: rules}
	exp.reductionEnabled = true
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}
