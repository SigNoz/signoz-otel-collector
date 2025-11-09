package signozspanmetricsconnector

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozspanmetricsconnector/internal/cache"
	"github.com/SigNoz/signoz-otel-collector/internal/coreinternal/traceutil"
	"github.com/jonboulle/clockwork"
	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

type connectorImp struct {
	lock       sync.Mutex
	logger     *zap.Logger
	instanceID string
	config     Config

	metricsConsumer consumer.Metrics

	// Additional dimensions to add to metrics
	dimensions             []dimension // signoz_latency metric
	expDimensions          []dimension // signoz_latency exphisto metric
	callDimensions         []dimension // signoz_calls_total metric
	dbCallDimensions       []dimension // signoz_db_latency_* metric
	externalCallDimensions []dimension // signoz_external_call_latency_* metric

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp

	// Histogram.
	histograms    map[metricKey]*histogramData // signoz_latency metric
	latencyBounds []float64
	expHistograms map[metricKey]*exponentialHistogram

	// TODO(nikhilmantri0902, srikanthccv): just like above, add support for expHistograms  for call, db and external Call histograms once we have
	// expHistograms ready for all
	callHistograms    map[metricKey]*histogramData // signoz_calls_total metric
	callLatencyBounds []float64

	dbCallHistograms    map[metricKey]*histogramData // signoz_db_latency_* metric
	dbCallLatencyBounds []float64

	externalCallHistograms    map[metricKey]*histogramData // signoz_external_call_latency_* metric
	externalCallLatencyBounds []float64

	keyBuf *bytes.Buffer

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions             *cache.Cache[metricKey, pcommon.Map]
	expHistogramKeyToDimensions       *cache.Cache[metricKey, pcommon.Map]
	callMetricKeyToDimensions         *cache.Cache[metricKey, pcommon.Map]
	dbCallMetricKeyToDimensions       *cache.Cache[metricKey, pcommon.Map]
	externalCallMetricKeyToDimensions *cache.Cache[metricKey, pcommon.Map]

	attrsCardinality    map[string]map[string]struct{}
	excludePatternRegex map[string]*regexp.Regexp

	wg           sync.WaitGroup
	clock        clockwork.Clock
	ticker       clockwork.Ticker
	done         chan struct{}
	started      bool
	shutdownOnce sync.Once

	serviceToOperations                    map[string]map[string]struct{}
	maxNumberOfServicesToTrack             int
	maxNumberOfOperationsToTrackPerService int
}

func newConnector(logger *zap.Logger, config component.Config, clock clockwork.Clock, instanceID string) (*connectorImp, error) {
	logger.Info("Building spanmetrics connector")
	cfg := config.(*Config)

	// Set default time bucket interval if not specified
	if cfg.TimeBucketInterval == 0 {
		cfg.TimeBucketInterval = defaultTimeBucketInterval
	}

	bounds := defaultLatencyHistogramBucketsMs
	if cfg.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(cfg.LatencyHistogramBuckets)
	}

	if err := validateDimensions(cfg.Dimensions, cfg.skipSanitizeLabel); err != nil {
		return nil, err
	}

	callDimensions := []Dimension{
		{Name: tagHTTPStatusCode},
	}
	callDimensions = append(callDimensions, cfg.Dimensions...)

	dbCallDimensions := []Dimension{
		{Name: conventions.AttributeDBSystem},
		{Name: conventions.AttributeDBName},
	}
	dbCallDimensions = append(dbCallDimensions, cfg.Dimensions...)

	var externalCallDimensions = []Dimension{
		{Name: tagHTTPStatusCode},
	}
	externalCallDimensions = append(externalCallDimensions, cfg.Dimensions...)

	if cfg.DimensionsCacheSize != 0 {
		logger.Warn("DimensionsCacheSize is deprecated, please use AggregationCardinalityLimit instead.")
	}

	metricKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	expHistogramKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	callMetricKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	dbMetricKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}
	externalCallMetricKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	excludePatternRegex := make(map[string]*regexp.Regexp)
	for _, pattern := range cfg.ExcludePatterns {
		excludePatternRegex[pattern.Name] = regexp.MustCompile(pattern.Pattern)
	}

	return &connectorImp{
		logger:     logger,
		config:     *cfg,
		instanceID: instanceID,

		dimensions:             newDimensions(cfg.Dimensions),
		expDimensions:          newDimensions(cfg.Dimensions),
		callDimensions:         newDimensions(callDimensions),
		dbCallDimensions:       newDimensions(dbCallDimensions),
		externalCallDimensions: newDimensions(externalCallDimensions),

		expHistograms:          make(map[metricKey]*exponentialHistogram),
		histograms:             make(map[metricKey]*histogramData),
		callHistograms:         make(map[metricKey]*histogramData),
		dbCallHistograms:       make(map[metricKey]*histogramData),
		externalCallHistograms: make(map[metricKey]*histogramData),

		serviceToOperations:                    make(map[string]map[string]struct{}),
		attrsCardinality:                       make(map[string]map[string]struct{}),
		maxNumberOfServicesToTrack:             cfg.MaxServicesToTrack,
		maxNumberOfOperationsToTrackPerService: cfg.MaxOperationsToTrackPerService,

		keyBuf: bytes.NewBuffer(make([]byte, 0, 1024)),

		metricKeyToDimensions:             metricKeyToDimensionsCache,
		expHistogramKeyToDimensions:       expHistogramKeyToDimensionsCache,
		callMetricKeyToDimensions:         callMetricKeyToDimensionsCache,
		dbCallMetricKeyToDimensions:       dbMetricKeyToDimensionsCache,
		externalCallMetricKeyToDimensions: externalCallMetricKeyToDimensionsCache,

		latencyBounds:             bounds,
		callLatencyBounds:         bounds,
		dbCallLatencyBounds:       bounds,
		externalCallLatencyBounds: bounds,

		startTimestamp:      pcommon.NewTimestampFromTime(time.Now()),
		clock:               clock,
		ticker:              clock.NewTicker(cfg.MetricsFlushInterval),
		done:                make(chan struct{}),
		excludePatternRegex: excludePatternRegex,
	}, nil
}

// Start implements the component.Component interface.
func (p *connectorImp) Start(ctx context.Context, _ component.Host) error {
	p.logger.Info("Starting signozspanmetricsprocessor with config", zap.Any("config", p.config))
	p.started = true
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-p.done:
				return
			case <-p.ticker.Chan():
				p.exportMetrics(ctx)
			}
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (p *connectorImp) Shutdown(context.Context) error {
	p.logger.Info("Shutting down signozspanmetricsconnector")
	p.shutdownOnce.Do(func() {
		if p.started {
			p.logger.Info("Stopping ticker")
			p.ticker.Stop()
			close(p.done)
			p.started = false
		}
	})
	p.wg.Wait()
	return nil
}

// Capabilities implements the consumer interface.
func (*connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics.
func (p *connectorImp) ConsumeTraces(_ context.Context, traces ptrace.Traces) error {
	p.lock.Lock()
	p.aggregateMetrics(traces)
	p.lock.Unlock()
	return nil
}

func (p *connectorImp) exportMetrics(ctx context.Context) {
	p.lock.Lock()

	m, err := p.buildMetrics()

	// Exemplars are only relevant to this batch of traces, so must be cleared within the lock,
	// regardless of error while building metrics, before the next batch of spans is received.
	p.resetExemplarData()

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	p.lock.Unlock()

	if err != nil {
		p.logCardinalityInfo()
		p.logger.Error("Failed to build metrics", zap.Error(err))
	}
	// TODO(srikanthccv): please verify as this is the place where a connector differs from a processor
	if err := p.metricsConsumer.ConsumeMetrics(ctx, m); err != nil {
		p.logger.Error("Failed ConsumeMetrics", zap.Error(err))
		return
	}
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *connectorImp) buildMetrics() (pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("signozspanmetricsprocessor")

	if err := p.collectCallMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}
	if err := p.collectLatencyMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}
	if err := p.collectExternalCallMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}
	if err := p.collectDBCallMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}

	if err := p.collectExpHistogramMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}

	p.metricKeyToDimensions.RemoveEvictedItems()
	p.expHistogramKeyToDimensions.RemoveEvictedItems()
	p.callMetricKeyToDimensions.RemoveEvictedItems()
	p.dbCallMetricKeyToDimensions.RemoveEvictedItems()
	p.externalCallMetricKeyToDimensions.RemoveEvictedItems()

	for key := range p.histograms {
		if !p.metricKeyToDimensions.Contains(key) {
			delete(p.histograms, key)
		}
	}
	for key := range p.callHistograms {
		if !p.callMetricKeyToDimensions.Contains(key) {
			delete(p.callHistograms, key)
		}
	}
	for key := range p.dbCallHistograms {
		if !p.dbCallMetricKeyToDimensions.Contains(key) {
			delete(p.dbCallHistograms, key)
		}
	}
	for key := range p.externalCallHistograms {
		if !p.externalCallMetricKeyToDimensions.Contains(key) {
			delete(p.externalCallHistograms, key)
		}
	}

	for key := range p.expHistograms {
		if !p.expHistogramKeyToDimensions.Contains(key) {
			delete(p.expHistograms, key)
		}
	}

	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.resetAccumulatedMetrics()
	}
	p.resetExemplarData()

	return m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *connectorImp) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	mLatency := ilm.Metrics().AppendEmpty()
	mLatency.SetName("signoz_latency")
	mLatency.SetUnit("ms")
	mLatency.SetEmptyHistogram().SetAggregationTemporality(p.config.GetAggregationTemporality())
	dps := mLatency.Histogram().DataPoints()
	dps.EnsureCapacity(len(p.histograms))

	for key, hist := range p.histograms {
		dpLatency := dps.AppendEmpty()
		start, end := parseTimesFromKeyOrNow(key, time.Now(), p.startTimestamp)
		dpLatency.SetStartTimestamp(start)
		dpLatency.SetTimestamp(end)
		dpLatency.ExplicitBounds().FromRaw(p.latencyBounds)
		dpLatency.BucketCounts().FromRaw(hist.bucketCounts)
		dpLatency.SetCount(hist.count)
		dpLatency.SetSum(hist.sum)
		setExemplars(hist.exemplarsData, end, dpLatency.Exemplars())

		dimensions, err := p.getDimensionsByMetricKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectExpHistogramMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *connectorImp) collectExpHistogramMetrics(ilm pmetric.ScopeMetrics) error {
	mExpLatency := ilm.Metrics().AppendEmpty()
	mExpLatency.SetName("signoz_latency")
	mExpLatency.SetUnit("ms")
	mExpLatency.SetEmptyExponentialHistogram().SetAggregationTemporality(p.config.GetAggregationTemporality())
	dps := mExpLatency.ExponentialHistogram().DataPoints()
	dps.EnsureCapacity(len(p.expHistograms))

	for key, hist := range p.expHistograms {
		dp := dps.AppendEmpty()
		start, end := parseTimesFromKeyOrNow(key, time.Now(), p.startTimestamp)
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		expoHistToExponentialDataPoint(hist.histogram, dp)
		dimensions, err := p.getDimensionsByExpHistogramKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dps.At(dps.Len() - 1).Attributes())
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *connectorImp) collectCallMetrics(ilm pmetric.ScopeMetrics) error {
	mCalls := ilm.Metrics().AppendEmpty()
	mCalls.SetName("signoz_calls_total")
	mCalls.SetEmptySum().SetIsMonotonic(true)
	mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())
	dps := mCalls.Sum().DataPoints()
	dps.EnsureCapacity(len(p.callHistograms))

	for key, hist := range p.callHistograms {
		dpCalls := dps.AppendEmpty()
		start, end := parseTimesFromKeyOrNow(key, time.Now(), p.startTimestamp)
		dpCalls.SetStartTimestamp(start)
		dpCalls.SetTimestamp(end)
		dpCalls.SetIntValue(int64(hist.count))

		dimensions, err := p.getDimensionsByCallMetricKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}
	return nil
}

// collectDBCallMetrics collects the raw latency sum and count metrics, writing the data
// into the given instrumentation library metrics.
func (p *connectorImp) collectDBCallMetrics(ilm pmetric.ScopeMetrics) error {
	mDBCallSum := ilm.Metrics().AppendEmpty()
	mDBCallSum.SetName("signoz_db_latency_sum")
	mDBCallSum.SetUnit("1")
	mDBCallSum.SetEmptySum().SetIsMonotonic(true)
	mDBCallSum.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

	mDBCallCount := ilm.Metrics().AppendEmpty()
	mDBCallCount.SetName("signoz_db_latency_count")
	mDBCallCount.SetUnit("1")
	mDBCallCount.SetEmptySum().SetIsMonotonic(true)
	mDBCallCount.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

	callSumDps := mDBCallSum.Sum().DataPoints()
	callCountDps := mDBCallCount.Sum().DataPoints()

	callSumDps.EnsureCapacity(len(p.dbCallHistograms))
	callCountDps.EnsureCapacity(len(p.dbCallHistograms))

	for key, metric := range p.dbCallHistograms {
		dpDBCallSum := callSumDps.AppendEmpty()
		start, end := parseTimesFromKeyOrNow(key, time.Now(), p.startTimestamp)
		dpDBCallSum.SetStartTimestamp(start)
		dpDBCallSum.SetTimestamp(end)
		dpDBCallSum.SetDoubleValue(metric.sum)

		dpDBCallCount := callCountDps.AppendEmpty()
		dpDBCallCount.SetStartTimestamp(start)
		dpDBCallCount.SetTimestamp(end)
		dpDBCallCount.SetIntValue(int64(metric.count))

		dimensions, err := p.getDimensionsByDBCallMetricKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dpDBCallSum.Attributes())
		dimensions.CopyTo(dpDBCallCount.Attributes())
	}
	return nil
}

func (p *connectorImp) collectExternalCallMetrics(ilm pmetric.ScopeMetrics) error {
	mExternalCallSum := ilm.Metrics().AppendEmpty()
	mExternalCallSum.SetName("signoz_external_call_latency_sum")
	mExternalCallSum.SetUnit("1")
	mExternalCallSum.SetEmptySum().SetIsMonotonic(true)
	mExternalCallSum.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

	mExternalCallCount := ilm.Metrics().AppendEmpty()
	mExternalCallCount.SetName("signoz_external_call_latency_count")
	mExternalCallCount.SetUnit("1")
	mExternalCallCount.SetEmptySum().SetIsMonotonic(true)
	mExternalCallCount.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

	callSumDps := mExternalCallSum.Sum().DataPoints()
	callCountDps := mExternalCallCount.Sum().DataPoints()

	callSumDps.EnsureCapacity(len(p.externalCallHistograms))
	callCountDps.EnsureCapacity(len(p.externalCallHistograms))

	for key, metric := range p.externalCallHistograms {
		dpExternalCallSum := callSumDps.AppendEmpty()
		start, end := parseTimesFromKeyOrNow(key, time.Now(), p.startTimestamp)
		dpExternalCallSum.SetStartTimestamp(start)
		dpExternalCallSum.SetTimestamp(end)
		dpExternalCallSum.SetDoubleValue(metric.sum)

		dpExternalCallCount := callCountDps.AppendEmpty()
		dpExternalCallCount.SetStartTimestamp(start)
		dpExternalCallCount.SetTimestamp(end)
		dpExternalCallCount.SetIntValue(int64(metric.count))

		dimensions, err := p.getDimensionsByExternalCallMetricKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dpExternalCallSum.Attributes())
		dimensions.CopyTo(dpExternalCallCount.Attributes())
	}
	return nil
}

// getDimensionsByMetricKey gets dimensions from `metricKeyToDimensions` cache.
func (p *connectorImp) getDimensionsByMetricKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.metricKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in metricKeyToDimensions cache by key %q", k)
}

// getDimensionsByExpHistogramKey gets dimensions from `expHistogramKeyToDimensions` cache.
func (p *connectorImp) getDimensionsByExpHistogramKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.expHistogramKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in expHistogramKeyToDimensions cache by key %q", k)
}

// callMetricKeyToDimensions gets dimensions from `callMetricKeyToDimensions` cache.
func (p *connectorImp) getDimensionsByCallMetricKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.callMetricKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in callMetricKeyToDimensions cache by key %q", k)
}

// getDimensionsByDBCallMetricKey gets dimensions from `dbCallMetricKeyToDimensions` cache.
func (p *connectorImp) getDimensionsByDBCallMetricKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.dbCallMetricKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in dbCallMetricKeyToDimensions cache by key %q", k)
}

// getDimensionsByExternalCallMetricKey gets dimensions from `externalCallMetricKeyToDimensions` cache.
func (p *connectorImp) getDimensionsByExternalCallMetricKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.externalCallMetricKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in externalCallMetricKeyToDimensions cache by key %q", k)
}

// resetExemplarData resets the entire exemplars map so the next trace will recreate all
// the data structure. An exemplar is a punctual value that exists at specific moment in time
// and should be not considered like a metrics that persist over time.
func (p *connectorImp) resetExemplarData() {
	for _, histo := range p.histograms {
		histo.exemplarsData = nil
	}
	for _, histo := range p.callHistograms {
		histo.exemplarsData = nil
	}
	for _, histo := range p.dbCallHistograms {
		histo.exemplarsData = nil
	}
	for _, histo := range p.externalCallHistograms {
		histo.exemplarsData = nil
	}
}

// resetAccumulatedMetrics resets the internal maps used to store created metric data. Also purge the cache for
// metricKeyToDimensions.
func (p *connectorImp) resetAccumulatedMetrics() {
	p.histograms = make(map[metricKey]*histogramData)
	p.expHistograms = make(map[metricKey]*exponentialHistogram)
	p.callHistograms = make(map[metricKey]*histogramData)
	p.dbCallHistograms = make(map[metricKey]*histogramData)
	p.externalCallHistograms = make(map[metricKey]*histogramData)

	p.metricKeyToDimensions.Purge()
	p.expHistogramKeyToDimensions.Purge()
	p.callMetricKeyToDimensions.Purge()
	p.dbCallMetricKeyToDimensions.Purge()
	p.externalCallMetricKeyToDimensions.Purge()
}

func (p *connectorImp) logCardinalityInfo() {
	for k, v := range p.attrsCardinality {
		values := make([]string, 0, len(v))
		for key := range v {
			values = append(values, key)
		}
		p.logger.Info("Attribute cardinality", zap.String("attribute", k), zap.Int("cardinality", len(v)))
		p.logger.Debug("Attribute values", zap.Strings("values", values))
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
//
// Metrics are grouped by resource attributes.
// Each metric is identified by a key that is built from the service name
// and span metadata such as name, kind, status_code and any additional
// dimensions the user has configured.
func (p *connectorImp) aggregateMetrics(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(serviceNameKey)
		if !ok {
			continue
		}
		resourceAttr.PutStr(signozID, p.instanceID)
		serviceName := serviceAttr.Str()
		ilsSlice := rspans.ScopeSpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			ils := ilsSlice.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				p.aggregateMetricsForSpan(serviceName, span, resourceAttr)
			}
		}
	}
}

func (p *connectorImp) aggregateMetricsForSpan(serviceName string, span ptrace.Span, resourceAttr pcommon.Map) {
	// Skip late-arriving spans older than the configured staleness spanSkipWindow
	if spanSkipWindow := p.config.GetSkipSpansOlderThan(); spanSkipWindow > 0 {
		lastPermissibleSpanTime := time.Now().Add(-spanSkipWindow)
		if span.StartTimestamp().AsTime().Before(lastPermissibleSpanTime) {
			p.logger.Debug("Skipping stale span", zap.String("span", span.Name()), zap.String("service", serviceName))
			return
		}
	}

	if p.shouldSkip(serviceName, span, resourceAttr) {
		p.logger.Debug("Skipping span", zap.String("span", span.Name()), zap.String("service", serviceName))
		return
	}
	// Protect against end timestamps before start timestamps. Assume 0 duration.
	latencyInMilliseconds := float64(0)
	startTime := span.StartTimestamp()
	endTime := span.EndTimestamp()
	if endTime > startTime {
		latencyInMilliseconds = float64(endTime-startTime) / float64(time.Millisecond.Nanoseconds())
	}

	// Build key for latency metrics (with conditional time bucketing)
	key := p.buildMetricKey(serviceName, span, p.dimensions, resourceAttr)
	p.cache(serviceName, span, key, resourceAttr)
	p.updateHistogram(key, latencyInMilliseconds, span.TraceID(), span.SpanID())

	if p.config.EnableExpHistogram {
		expKey := p.buildMetricKey(serviceName, span, p.expDimensions, resourceAttr)
		p.expHistogramCache(serviceName, span, expKey, resourceAttr)
		p.updateExpHistogram(expKey, latencyInMilliseconds, span.TraceID(), span.SpanID())
	}

	// Build key for call metrics (with conditional time bucketing)
	callKey := p.buildMetricKey(serviceName, span, p.callDimensions, resourceAttr)
	p.callCache(serviceName, span, callKey, resourceAttr)
	p.updateCallHistogram(callKey, latencyInMilliseconds, span.TraceID(), span.SpanID())

	spanAttr := span.Attributes()
	remoteAddr, externalCallPresent := getRemoteAddress(span)

	if span.Kind() == ptrace.SpanKindClient && externalCallPresent {
		extraVals := []string{remoteAddr}
		externalCallKey := p.buildCustomMetricKey(serviceName, span, p.externalCallDimensions, resourceAttr, extraVals)
		extraDims := map[string]pcommon.Value{
			"address": pcommon.NewValueStr(remoteAddr),
		}
		p.externalCallCache(serviceName, span, externalCallKey, resourceAttr, extraDims)
		p.updateExternalHistogram(externalCallKey, latencyInMilliseconds, span.TraceID(), span.SpanID())
	}

	_, dbCallPresent := spanAttr.Get("db.system")
	if span.Kind() != ptrace.SpanKindServer && dbCallPresent {
		dbCallKey := p.buildCustomMetricKey(serviceName, span, p.dbCallDimensions, resourceAttr, nil)
		p.dbCallCache(serviceName, span, dbCallKey, resourceAttr)
		p.updateDBHistogram(dbCallKey, latencyInMilliseconds, span.TraceID(), span.SpanID())
	}
}

// updateHistogram adds the histogram sample to the histogram defined by the metric key.
func (p *connectorImp) updateHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.histograms[key]
	if !ok {
		histo = &histogramData{
			bucketCounts: make([]uint64, len(p.latencyBounds)+1),
		}
		p.histograms[key] = histo
	}

	histo.sum += latency
	histo.count++
	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)
	histo.bucketCounts[index]++
	histo.exemplarsData = append(histo.exemplarsData, exemplarData{traceID: traceID, spanID: spanID, value: latency})
}

func (p *connectorImp) updateExpHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.expHistograms[key]
	if !ok {
		histogram := new(structure.Histogram[float64])
		cfg := structure.NewConfig(
			structure.WithMaxSize(structure.DefaultMaxSize),
		)
		histogram.Init(cfg)

		histo = &exponentialHistogram{
			histogram: histogram,
		}
		p.expHistograms[key] = histo
	}

	histo.Observe(latency)
}

func (p *connectorImp) updateCallHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.callHistograms[key]
	if !ok {
		histo = &histogramData{
			bucketCounts: make([]uint64, len(p.latencyBounds)+1),
		}
		p.callHistograms[key] = histo
	}

	histo.sum += latency
	histo.count++
	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)
	histo.bucketCounts[index]++
	histo.exemplarsData = append(histo.exemplarsData, exemplarData{traceID: traceID, spanID: spanID, value: latency})
}

func (p *connectorImp) updateDBHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.dbCallHistograms[key]
	if !ok {
		histo = &histogramData{
			bucketCounts: make([]uint64, len(p.latencyBounds)+1),
		}
		p.dbCallHistograms[key] = histo
	}

	histo.sum += latency
	histo.count++
	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)
	histo.bucketCounts[index]++
	histo.exemplarsData = append(histo.exemplarsData, exemplarData{traceID: traceID, spanID: spanID, value: latency})
}

func (p *connectorImp) updateExternalHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.externalCallHistograms[key]
	if !ok {
		histo = &histogramData{
			bucketCounts: make([]uint64, len(p.latencyBounds)+1),
		}
		p.externalCallHistograms[key] = histo
	}

	histo.sum += latency
	histo.count++
	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)
	histo.bucketCounts[index]++
	histo.exemplarsData = append(histo.exemplarsData, exemplarData{traceID: traceID, spanID: spanID, value: latency})
}

func (p *connectorImp) shouldSkip(serviceName string, span ptrace.Span, resourceAttrs pcommon.Map) bool {
	for key, pattern := range p.excludePatternRegex {
		if key == serviceNameKey && pattern.MatchString(serviceName) {
			return true
		}
		if key == operationKey && pattern.MatchString(span.Name()) {
			return true
		}
		if key == spanKindKey && pattern.MatchString(span.Kind().String()) {
			return true
		}
		if key == statusCodeKey && pattern.MatchString(span.Status().Code().String()) {
			return true
		}

		matched := false
		span.Attributes().Range(func(k string, v pcommon.Value) bool {
			if key == k && pattern.MatchString(v.AsString()) {
				matched = true
			}
			return true
		})
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			if key == k && pattern.MatchString(v.AsString()) {
				matched = true
			}
			return true
		})
		if matched {
			return true
		}
	}
	return false
}

// addTimeToKeyBuf writes a time-bucket prefix to the key buffer.
// It truncates the provided time to the configured bucket interval before writing.
func (p *connectorImp) addTimeToKeyBuf(t time.Time) {
	bucket := t.Truncate(p.config.GetTimeBucketInterval())
	p.keyBuf.WriteString(strconv.FormatInt(bucket.Unix(), 10))
	p.keyBuf.WriteString(metricKeySeparator)
}

// buildMetricKey builds a metric key with optional time bucketing based on temporality.
// For delta temporality: prepends time bucket prefix
// For cumulative temporality: no time bucketing (to avoid memory issues)
func (p *connectorImp) buildMetricKey(serviceName string, span ptrace.Span, dimensions []dimension, resourceAttr pcommon.Map) metricKey {
	p.keyBuf.Reset()

	// Only add time bucket prefix for delta temporality
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.addTimeToKeyBuf(span.StartTimestamp().AsTime())
	}

	p.buildKey(p.keyBuf, serviceName, span, dimensions, resourceAttr)
	return metricKey(p.keyBuf.String())
}

// buildCustomMetricKey builds a custom metric key with optional time bucketing based on temporality.
// For delta temporality: prepends time bucket prefix
// For cumulative temporality: no time bucketing (to avoid memory issues)
func (p *connectorImp) buildCustomMetricKey(serviceName string, span ptrace.Span, dimensions []dimension, resourceAttr pcommon.Map, extraVals []string) metricKey {
	p.keyBuf.Reset()

	// Only add time bucket prefix for delta temporality
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.addTimeToKeyBuf(span.StartTimestamp().AsTime())
	}

	p.buildCustomKey(p.keyBuf, serviceName, span, dimensions, resourceAttr, extraVals)
	return metricKey(p.keyBuf.String())
}

// buildKey builds the metric key from the service name and span metadata such as name, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource/event attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// buildKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func (p *connectorImp) buildKey(dest *bytes.Buffer, serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map) {
	spanName := span.Name()
	if len(p.serviceToOperations) > p.maxNumberOfServicesToTrack {
		p.logger.Warn("Too many services to track, using overflow service name",
			zap.Int("maxNumberOfServicesToTrack", p.maxNumberOfServicesToTrack))
		serviceName = overflowServiceName
	}
	if len(p.serviceToOperations[serviceName]) > p.maxNumberOfOperationsToTrackPerService {
		p.logger.Warn("Too many operations to track, using overflow operation name",
			zap.Int("maxNumberOfOperationsToTrackPerService", p.maxNumberOfOperationsToTrackPerService),
			zap.String("serviceName", serviceName))
		spanName = overflowOperation
	}
	concatDimensionValue(dest, serviceName, false)
	concatDimensionValue(dest, spanName, true)
	concatDimensionValue(dest, traceutil.SpanKindStr(span.Kind()), true)
	concatDimensionValue(dest, traceutil.StatusCodeStr(span.Status().Code()), true)

	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			concatDimensionValue(dest, v.AsString(), true)
		}
	}
	if _, ok := p.serviceToOperations[serviceName]; !ok {
		p.serviceToOperations[serviceName] = make(map[string]struct{})
	}
	p.serviceToOperations[serviceName][spanName] = struct{}{}

}

// TODO(nikhilmantri0902, srikanthccv): In the function buildKey we check the maxNumberOfServicesToTrack and maxNumberOfOperationsToTrackPerService caps.
// But buildCustomKey we do not check these caps. This is because for DB and external call metrics we do not have spanNames in consideration yet.
// Goig forward, we will consider spanName for DB and external call metrics also, and hence we will be adding these cap checks in buildCustomKey.
// this same logic applies in buildDimensionKVs and buildCustomDimensionKVs.

// buildCustomKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func (p *connectorImp) buildCustomKey(dest *bytes.Buffer, serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map, extraVals []string) {
	concatDimensionValue(dest, serviceName, false)
	concatDimensionValue(dest, traceutil.StatusCodeStr(span.Status().Code()), true)
	for _, val := range extraVals {
		concatDimensionValue(dest, val, true)
	}

	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValueWithResource(d, span.Attributes(), resourceAttrs)
		if ok {
			concatDimensionValue(dest, v.AsString(), true)
		}

		if foundInResource {
			concatDimensionValue(dest, resourcePrefix+v.AsString(), true)
		}
	}
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *connectorImp) cache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.metricKeyToDimensions.Get(k); !has {
		p.metricKeyToDimensions.Add(k, p.buildDimensionKVs(serviceName, span, p.dimensions, resourceAttrs))
	}
}

// expHistogramCache caches the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.expHistogramKeyToDimensions[key])
func (p *connectorImp) expHistogramCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.expHistogramKeyToDimensions.Get(k); !has {
		p.expHistogramKeyToDimensions.Add(k, p.buildDimensionKVs(serviceName, span, p.expDimensions, resourceAttrs))
	}
}

// callCache caches the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.callMetricKeyToDimensions[key])
func (p *connectorImp) callCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.callMetricKeyToDimensions.Get(k); !has {
		p.callMetricKeyToDimensions.Add(k, p.buildDimensionKVs(serviceName, span, p.callDimensions, resourceAttrs))
	}
}

// dbCallCache caches the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.dbCallMetricKeyToDimensions[key])
func (p *connectorImp) dbCallCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.dbCallMetricKeyToDimensions.Get(k); !has {
		p.dbCallMetricKeyToDimensions.Add(k, p.buildCustomDimensionKVs(serviceName, span, p.dbCallDimensions, resourceAttrs, nil))
	}
}

// externalCallCache caches the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.externalCallMetricKeyToDimensions[key])
func (p *connectorImp) externalCallCache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map, extraDims map[string]pcommon.Value) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.externalCallMetricKeyToDimensions.Get(k); !has {
		p.externalCallMetricKeyToDimensions.Add(k, p.buildCustomDimensionKVs(serviceName, span, p.externalCallDimensions, resourceAttrs, extraDims))
	}
}

func (p *connectorImp) buildDimensionKVs(serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map) pcommon.Map {

	spanName := span.Name()
	if len(p.serviceToOperations) > p.maxNumberOfServicesToTrack {
		serviceName = overflowServiceName
	}
	if len(p.serviceToOperations[serviceName]) > p.maxNumberOfOperationsToTrackPerService {
		spanName = overflowOperation
	}

	dims := pcommon.NewMap()
	dims.EnsureCapacity(4 + len(optionalDims))
	dims.PutStr(serviceNameKey, serviceName)
	dims.PutStr(operationKey, spanName)
	dims.PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	dims.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValueWithResource(d, span.Attributes(), resourceAttrs)
		if ok {
			v.CopyTo(dims.PutEmpty(d.name))
		}
		if foundInResource {
			v.CopyTo(dims.PutEmpty(resourcePrefix + d.name))
		}
	}
	dims.Range(func(k string, v pcommon.Value) bool {
		if _, exists := p.attrsCardinality[k]; !exists {
			p.attrsCardinality[k] = make(map[string]struct{})
		}
		p.attrsCardinality[k][v.AsString()] = struct{}{}
		return true
	})
	return dims
}

func (p *connectorImp) buildCustomDimensionKVs(serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map, extraDims map[string]pcommon.Value) pcommon.Map {
	dims := pcommon.NewMap()

	dims.PutStr(serviceNameKey, serviceName)
	for k, v := range extraDims {
		v.CopyTo(dims.PutEmpty(k))
	}
	dims.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))

	for _, d := range optionalDims {
		v, ok, foundInResource := getDimensionValueWithResource(d, span.Attributes(), resourceAttrs)
		if ok {
			v.CopyTo(dims.PutEmpty(d.name))
		}
		if foundInResource {
			v.CopyTo(dims.PutEmpty(resourcePrefix + d.name))
		}
	}
	dims.Range(func(k string, v pcommon.Value) bool {
		if _, exists := p.attrsCardinality[k]; !exists {
			p.attrsCardinality[k] = make(map[string]struct{})
		}
		p.attrsCardinality[k][v.AsString()] = struct{}{}
		return true
	})
	return dims
}
