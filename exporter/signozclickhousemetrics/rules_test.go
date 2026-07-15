package signozclickhousemetrics

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
)

func reductionTestConfig() *Config {
	return &Config{
		MetadataWriteSampleRatio: 1.0,
		Reduction: ReductionConfig{
			Enabled:               true,
			PollInterval:          time.Minute,
			RulesTable:            "distributed_metric_reduction_rules",
			BufferSamplesTable:    "distributed_samples_v4_buffer",
			BufferTimeSeriesTable: "distributed_time_series_v4_buffer",
		},
	}
}

func newReductionExporter(t *testing.T, rules ruleSet) *clickhouseMetricsExporter {
	t.Helper()
	exp, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(reductionTestConfig()),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	exp.reductionRules.Store(&rules)
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
	exp := newReductionExporter(t, ruleSet{
		"system.memory.usage0": {keys: labelSet("gauge.attr_0"), keep: true},
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
	exp := newReductionExporter(t, ruleSet{
		"system.memory.usage0": {keys: labelSet("resource.attr_0")},
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
	exp := newReductionExporter(t, ruleSet{
		"system.memory.usage0": {keys: labelSet("resource.attr_0"), effectiveFromUnixMilli: 1727286182001},
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
	exp := newReductionExporter(t, ruleSet{
		"some.other.metric": {keys: labelSet("resource.attr_0")},
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
	rules := ruleSet{"system.memory.usage0": {keys: labelSet("resource.attr_0")}}
	disabled.reductionRules.Store(&rules)
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

	exp := newReductionExporter(t, ruleSet{
		"http.server.requests.count": {keys: labelSet("service.instance.id")},
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
	exp := newReductionExporter(t, ruleSet{
		"http.server.duration0.bucket": {keys: labelSet("resource.attr_0")},
		"http.server.duration0.count":  {keys: labelSet("resource.attr_0")},
	})
	metrics := pmetricsgen.GenerateHistogramMetrics(1, 1, 1, 1, 1, 0, 0)
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
	rules := ruleSet{
		"http.server.duration1": {keys: labelSet("resource.attr_0")},
	}
	exp.reductionRules.Store(&rules)

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

func Test_reductionRuleAppliesAt(t *testing.T) {
	rule := &reductionRule{keys: labelSet("a"), effectiveFromUnixMilli: 100}
	assert.False(t, rule.appliesAt(99))
	assert.True(t, rule.appliesAt(100))
	assert.True(t, rule.appliesAt(101))
	assert.True(t, rule.drop("a"))
	assert.False(t, rule.drop("b"))
}

func Test_collectUsageForSample(t *testing.T) {
	exp := newReductionExporter(t, ruleSet{
		"http.server.duration0.bucket": {keys: labelSet("resource.attr_0")},
	})

	assert.True(t, exp.collectUsageForSample(&sample{metricName: "http.server.duration0.count"}))

	// internal metrics
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "signoz.collector.foo"}))
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "chi.foo"}))
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "otelcol.foo"}))

	// reduced
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "anything", reducedFingerprint: 42}))

	// ruled metric is skipped
	assert.False(t, exp.collectUsageForSample(&sample{metricName: "http.server.duration0.bucket"}))

	disabled, err := NewClickHouseExporter(
		WithLogger(zap.NewNop()),
		WithConfig(&Config{}),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
	)
	require.NoError(t, err)
	assert.True(t, disabled.collectUsageForSample(&sample{metricName: "http.server.duration0.bucket"}))
}

func Test_reductionConfigValidate(t *testing.T) {
	cfg := reductionTestConfig()
	cfg.DSN = "tcp://localhost:9000"
	require.NoError(t, cfg.Validate())

	bad := *cfg
	bad.Reduction.PollInterval = time.Second
	require.Error(t, bad.Validate())

	bad = *cfg
	bad.Reduction.RulesTable = ""
	require.Error(t, bad.Validate())

	bad = *cfg
	bad.Reduction.BufferSamplesTable = ""
	require.Error(t, bad.Validate())

	bad = *cfg
	bad.Reduction.BufferTimeSeriesTable = ""
	require.Error(t, bad.Validate())
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
	rules := ruleSet{"system.memory.usage0": {keys: labelSet("resource.attr_0")}}
	exp.reductionRules.Store(&rules)

	metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
	require.NoError(t, exp.PushMetrics(context.Background(), metrics))
	require.NoError(t, exp.Shutdown(context.Background()))
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
	rules := make(ruleSet, 10000)
	for i := 0; i < 10000; i++ {
		rules["system.memory.usage"+strconv.Itoa(i)] = &reductionRule{keys: labelSet("resource.attr_0")}
	}
	exp.reductionRules.Store(&rules)
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
	rules := make(ruleSet, numMetrics)
	for i := 0; i < numMetrics; i++ {
		rules["system.memory.usage"+strconv.Itoa(i)] = &reductionRule{keys: labelSet("resource.attr_0")}
	}
	exp.reductionRules.Store(&rules)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
}
