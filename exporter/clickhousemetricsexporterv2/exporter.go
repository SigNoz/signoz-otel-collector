package clickhousemetricsexporterv2

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.opentelemetry.io/otel/attribute"
	metricapi "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

var (
	resourceAttrType  = "resource"
	scopeAttrType     = "scope"
	pointAttrType     = "point"
	countSuffix       = ".count"
	sumSuffix         = ".sum"
	minSuffix         = ".min"
	maxSuffix         = ".max"
	bucketSuffix      = ".bucket"
	quantilesSuffix   = ".quantile"
	samplesSQLTmpl    = "INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, value) VALUES (?, ?, ?, ?, ?, ?)"
	timeSeriesSQLTmpl = "INSERT INTO %s.%s (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	expHistSQLTmpl    = "INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, count, sum, min, max, sketch) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	metadataSQLTmpl   = "INSERT INTO %s.%s (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	meterScope        = "github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporterv2"
)

type clickhouseMetricsExporter struct {
	cfg           *Config
	logger        *zap.Logger
	meter         metricapi.Meter
	cache         *ttlcache.Cache[string, bool]
	conn          clickhouse.Conn
	wg            sync.WaitGroup
	enableExpHist bool

	samplesSQL    string
	timeSeriesSQL string
	expHistSQL    string
	metadataSQL   string

	processMetricsDuration metricapi.Float64Histogram
	exportMetricsDuration  metricapi.Float64Histogram
}

// sample represents a single metric sample
// directly mapped to the table `samples_v4` schema
type sample struct {
	env         string
	temporality pmetric.AggregationTemporality
	metricName  string
	fingerprint uint64
	unixMilli   int64
	value       float64
}

// exponentialHistogramSample represents a single exponential histogram sample
// directly mapped to the table `exp_hist` schema
type exponentialHistogramSample struct {
	env         string
	temporality pmetric.AggregationTemporality
	metricName  string
	fingerprint uint64
	unixMilli   int64
	sketch      chproto.DD
	count       float64
	sum         float64
	min         float64
	max         float64
}

// ts represents a single time series
// directly mapped to the table `time_series_v4` schema
type ts struct {
	env           string
	temporality   pmetric.AggregationTemporality
	metricName    string
	description   string
	unit          string
	typ           pmetric.MetricType
	isMonotonic   bool
	fingerprint   uint64
	unixMilli     int64
	labels        string
	attrs         map[string]string
	scopeAttrs    map[string]string
	resourceAttrs map[string]string
}

// metadata represents a single metric metadata
// directly mapped to the table `metadata` schema
type metadata struct {
	metricName      string
	temporality     pmetric.AggregationTemporality
	description     string
	unit            string
	typ             pmetric.MetricType
	isMonotonic     bool
	attrName        string
	attrType        string
	attrDatatype    pcommon.ValueType
	attrStringValue string
}

// writeBatch is a batch of data to be written to the database
type writeBatch struct {
	samples  []sample
	expHist  []exponentialHistogramSample
	ts       []ts
	metadata []metadata

	metaSeen map[string]struct{}
}

type ExporterOption func(e *clickhouseMetricsExporter) error

func WithLogger(logger *zap.Logger) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.logger = logger
		return nil
	}
}

func WithMeter(meter metricapi.Meter) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.meter = meter
		return nil
	}
}

func WithEnableExpHist(enableExpHist bool) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.enableExpHist = enableExpHist
		return nil
	}
}

func WithCache(cache *ttlcache.Cache[string, bool]) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.cache = cache
		return nil
	}
}

func WithConn(conn clickhouse.Conn) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.conn = conn
		return nil
	}
}

func WithConfig(cfg *Config) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.cfg = cfg
		return nil
	}
}

func defaultOptions() []ExporterOption {

	cache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](45*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)
	go cache.Start()

	return []ExporterOption{
		WithCache(cache),
		WithLogger(zap.NewNop()),
		WithEnableExpHist(false),
		WithMeter(noop.NewMeterProvider().Meter(meterScope)),
	}
}

func NewClickHouseExporter(opts ...ExporterOption) (*clickhouseMetricsExporter, error) {

	chExporter := &clickhouseMetricsExporter{}

	newOptions := append(defaultOptions(), opts...)

	for _, opt := range newOptions {
		if err := opt(chExporter); err != nil {
			return nil, err
		}
	}

	chExporter.samplesSQL = fmt.Sprintf(samplesSQLTmpl, chExporter.cfg.Database, chExporter.cfg.SamplesTable)
	chExporter.timeSeriesSQL = fmt.Sprintf(timeSeriesSQLTmpl, chExporter.cfg.Database, chExporter.cfg.TimeSeriesTable)
	chExporter.expHistSQL = fmt.Sprintf(expHistSQLTmpl, chExporter.cfg.Database, chExporter.cfg.ExpHistTable)
	chExporter.metadataSQL = fmt.Sprintf(metadataSQLTmpl, chExporter.cfg.Database, chExporter.cfg.MetadataTable)

	var err error
	chExporter.processMetricsDuration, err = chExporter.meter.Float64Histogram(
		"exporter_prepare_metrics_duration",
		metricapi.WithDescription("Time taken (in millis) for exporter to prepare metrics"),
	)
	if err != nil {
		return nil, err
	}

	chExporter.exportMetricsDuration, err = chExporter.meter.Float64Histogram(
		"exporter_db_write_latency",
		metricapi.WithDescription("Time taken to write data to ClickHouse"),
		metricapi.WithUnit("ms"),
		metricapi.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
		return nil, err
	}

	return chExporter, nil
}

func (c *clickhouseMetricsExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (c *clickhouseMetricsExporter) Shutdown(ctx context.Context) error {
	c.wg.Wait()
	return c.conn.Close()
}

func (c *clickhouseMetricsExporter) processMetadata(
	batch *writeBatch, name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, attrs pcommon.Map, attrType string) {
	attrs.Range(func(key string, value pcommon.Value) bool {
		// there should never be a conflicting key (either with resource, scope, or point attributes) in metrics
		// it breaks the fingerprinting, we assume this will never happen
		// even if it does, we will not handle it on our end (because we can't reliably which should take
		// precedence), the user should be responsible for ensuring no conflicting keys in their metrics
		if _, ok := batch.metaSeen[key]; ok {
			return true
		}
		batch.metaSeen[key] = struct{}{}
		batch.metadata = append(batch.metadata, metadata{
			metricName:      name,
			temporality:     temporality,
			description:     desc,
			unit:            unit,
			typ:             typ,
			isMonotonic:     isMonotonic,
			attrName:        key,
			attrType:        attrType,
			attrDatatype:    value.Type(),
			attrStringValue: value.AsString(),
		})
		return true
	})
}

// processGauge processes gauge metrics
func (c *clickhouseMetricsExporter) processGauge(batch *writeBatch, metric pmetric.Metric, resAttrs pcommon.Map, scopeAttrs pcommon.Map) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// gauge metrics do not have a temporality
	temporality := pmetric.AggregationTemporalityUnspecified
	env := ""
	if de, ok := resAttrs.Get(semconv.AttributeDeploymentEnvironment); ok {
		env = de.AsString()
	}
	// there is no monotonicity for gauge metrics
	isMonotonic := false

	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, resAttrs, resourceAttrType)
	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, scopeAttrs, scopeAttrType)

	for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
		dp := metric.Gauge().DataPoints().At(i)
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := dp.Attributes()
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:   unixMilli,
			value:       value,
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}
}

// processSum processes sum metrics
func (c *clickhouseMetricsExporter) processSum(batch *writeBatch, metric pmetric.Metric, resAttrs pcommon.Map, scopeAttrs pcommon.Map) {

	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// sum metrics have a temporality
	temporality := metric.Sum().AggregationTemporality()
	env := ""
	if de, ok := resAttrs.Get(semconv.AttributeDeploymentEnvironment); ok {
		env = de.AsString()
	}
	isMonotonic := metric.Sum().IsMonotonic()

	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, resAttrs, resourceAttrType)
	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, scopeAttrs, scopeAttrType)

	for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
		dp := metric.Sum().DataPoints().At(i)
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := dp.Attributes()
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:   unixMilli,
			value:       value,
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}
}

// processHistogram processes histogram metrics
func (c *clickhouseMetricsExporter) processHistogram(batch *writeBatch, metric pmetric.Metric, resAttrs pcommon.Map, scopeAttrs pcommon.Map) {

	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	temporality := metric.Histogram().AggregationTemporality()
	env := ""
	if de, ok := resAttrs.Get(semconv.AttributeDeploymentEnvironment); ok {
		env = de.AsString()
	}
	// monotonicity is assumed for histograms
	isMonotonic := true

	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, resAttrs, resourceAttrType)
	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, scopeAttrs, scopeAttrType)

	addSample := func(batch *writeBatch, dp pmetric.HistogramDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
		case sumSuffix:
			value = dp.Sum()
		case minSuffix:
			value = dp.Min()
		case maxSuffix:
			value = dp.Max()
		}
		pointAttrs := dp.Attributes()
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:   unixMilli,
			value:       value,
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}

	addBucketSample := func(batch *writeBatch, dp pmetric.HistogramDataPoint, suffix string) {
		var cumulativeCount uint64
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := pcommon.NewMap()
		dp.Attributes().CopyTo(pointAttrs)

		for i := 0; i < dp.ExplicitBounds().Len() && i < dp.BucketCounts().Len(); i++ {
			bound := dp.ExplicitBounds().At(i)
			cumulativeCount += dp.BucketCounts().At(i)
			boundStr := strconv.FormatFloat(bound, 'f', -1, 64)
			pointAttrs.PutStr("le", boundStr)

			batch.samples = append(batch.samples, sample{
				env:         env,
				temporality: temporality,
				metricName:  name + suffix,
				fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
				unixMilli:   unixMilli,
				value:       float64(cumulativeCount),
			})
			c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

			batch.ts = append(batch.ts, ts{
				env:           env,
				temporality:   temporality,
				metricName:    name + suffix,
				description:   desc,
				unit:          unit,
				typ:           typ,
				isMonotonic:   isMonotonic,
				fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
				unixMilli:     unixMilli,
				labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
				attrs:         getAttrMap(pointAttrs),
				scopeAttrs:    getAttrMap(scopeAttrs),
				resourceAttrs: getAttrMap(resAttrs),
			})
		}
		// add le=+Inf sample
		pointAttrs.PutStr("le", "+Inf")
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:   unixMilli,
			value:       float64(dp.Count()),
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)
		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}

	for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
		dp := metric.Histogram().DataPoints().At(i)
		// we need to create five samples for each histogram dp
		// 1. count
		// 2. sum
		// 3. min
		// 4. max
		// 5. bucket counts
		addSample(batch, dp, countSuffix)
		addSample(batch, dp, sumSuffix)
		addSample(batch, dp, minSuffix)
		addSample(batch, dp, maxSuffix)
		addBucketSample(batch, dp, bucketSuffix)
	}
}

func (c *clickhouseMetricsExporter) processSummary(batch *writeBatch, metric pmetric.Metric, resAttrs pcommon.Map, scopeAttrs pcommon.Map) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	temporality := pmetric.AggregationTemporalityUnspecified
	env := ""
	if de, ok := resAttrs.Get(semconv.AttributeDeploymentEnvironment); ok {
		env = de.AsString()
	}
	// monotonicity is assumed for summaries
	isMonotonic := true

	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, resAttrs, resourceAttrType)
	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, scopeAttrs, scopeAttrType)

	addSample := func(batch *writeBatch, dp pmetric.SummaryDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
		case sumSuffix:
			value = dp.Sum()
		}
		pointAttrs := dp.Attributes()
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:   unixMilli,
			value:       value,
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}

	addQuantileSample := func(batch *writeBatch, dp pmetric.SummaryDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := pcommon.NewMap()
		dp.Attributes().CopyTo(pointAttrs)
		for i := 0; i < dp.QuantileValues().Len(); i++ {
			quantile := dp.QuantileValues().At(i)
			quantileStr := strconv.FormatFloat(quantile.Quantile(), 'f', -1, 64)
			quantileValue := quantile.Value()
			pointAttrs.PutStr("quantile", quantileStr)
			batch.samples = append(batch.samples, sample{
				env:         env,
				temporality: temporality,
				metricName:  name + suffix,
				fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
				unixMilli:   unixMilli,
				value:       quantileValue,
			})
			c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)
			batch.ts = append(batch.ts, ts{
				env:           env,
				temporality:   temporality,
				metricName:    name + suffix,
				description:   desc,
				unit:          unit,
				typ:           typ,
				isMonotonic:   isMonotonic,
				fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
				unixMilli:     unixMilli,
				labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
				attrs:         getAttrMap(pointAttrs),
				scopeAttrs:    getAttrMap(scopeAttrs),
				resourceAttrs: getAttrMap(resAttrs),
			})
		}
	}

	for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
		dp := metric.Summary().DataPoints().At(i)
		// for summary metrics, we need to create three samples
		// 1. count
		// 2. sum
		// 3. quantiles
		addSample(batch, dp, countSuffix)
		addSample(batch, dp, sumSuffix)
		addQuantileSample(batch, dp, quantilesSuffix)
	}
}

func (c *clickhouseMetricsExporter) processExponentialHistogram(batch *writeBatch, metric pmetric.Metric, resAttrs pcommon.Map, scopeAttrs pcommon.Map) {
	if !c.enableExpHist {
		c.logger.Debug("exponential histogram is not enabled")
		return
	}

	if metric.ExponentialHistogram().AggregationTemporality() != pmetric.AggregationTemporalityDelta {
		c.logger.Warn("exponential histogram temporality is not delta", zap.String("metric_name", metric.Name()), zap.String("temporality", metric.ExponentialHistogram().AggregationTemporality().String()))
		return
	}

	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	temporality := metric.ExponentialHistogram().AggregationTemporality()

	env := ""
	if de, ok := resAttrs.Get(semconv.AttributeDeploymentEnvironment); ok {
		env = de.AsString()
	}

	isMonotonic := true

	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, resAttrs, resourceAttrType)
	c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, scopeAttrs, scopeAttrType)

	addSample := func(batch *writeBatch, dp pmetric.ExponentialHistogramDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
		case sumSuffix:
			value = dp.Sum()
		case minSuffix:
			value = dp.Min()
		case maxSuffix:
			value = dp.Max()
		}
		pointAttrs := dp.Attributes()
		batch.samples = append(batch.samples, sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:   unixMilli,
			value:       value,
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name+suffix),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name+suffix)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}

	toStore := func(buckets pmetric.ExponentialHistogramDataPointBuckets) *chproto.Store {
		bincounts := make([]float64, 0, buckets.BucketCounts().Len())
		for _, bucket := range buckets.BucketCounts().AsRaw() {
			bincounts = append(bincounts, float64(bucket))
		}

		store := &chproto.Store{
			ContiguousBinIndexOffset: int32(buckets.Offset()),
			ContiguousBinCounts:      bincounts,
		}
		return store
	}

	addDDSketchSample := func(batch *writeBatch, dp pmetric.ExponentialHistogramDataPoint) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := dp.Attributes()
		positive := toStore(dp.Positive())
		negative := toStore(dp.Negative())
		gamma := math.Pow(2, math.Pow(2, float64(-dp.Scale())))
		dd := chproto.DD{
			Mapping:        &chproto.IndexMapping{Gamma: gamma},
			PositiveValues: positive,
			NegativeValues: negative,
			ZeroCount:      float64(dp.ZeroCount()),
		}
		batch.expHist = append(batch.expHist, exponentialHistogramSample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:   unixMilli,
			sketch:      dd,
			count:       float64(dp.Count()),
			sum:         dp.Sum(),
			min:         dp.Min(),
			max:         dp.Max(),
		})
		c.processMetadata(batch, name, desc, unit, typ, temporality, isMonotonic, pointAttrs, pointAttrType)

		batch.ts = append(batch.ts, ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   Fingerprint(pointAttrs, scopeAttrs, resAttrs, name),
			unixMilli:     unixMilli,
			labels:        getJSONString(getAllLabels(pointAttrs, scopeAttrs, resAttrs, name)),
			attrs:         getAttrMap(pointAttrs),
			scopeAttrs:    getAttrMap(scopeAttrs),
			resourceAttrs: getAttrMap(resAttrs),
		})
	}

	for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
		dp := metric.ExponentialHistogram().DataPoints().At(i)
		// we need to create five samples for each exponential histogram dp
		// 1. count
		// 2. sum
		// 3. min
		// 4. max
		// 5. ddsketch
		addSample(batch, dp, countSuffix)
		addSample(batch, dp, sumSuffix)
		addSample(batch, dp, minSuffix)
		addSample(batch, dp, maxSuffix)
		addDDSketchSample(batch, dp)
	}

}

func (c *clickhouseMetricsExporter) prepareBatch(md pmetric.Metrics) *writeBatch {
	batch := &writeBatch{metaSeen: make(map[string]struct{})}
	start := time.Now()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttrs := pcommon.NewMap()
		rm.Resource().Attributes().CopyTo(resAttrs)
		resAttrs.PutStr("__resource.schema_url__", rm.SchemaUrl())
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeAttrs := pcommon.NewMap()
			sm.Scope().Attributes().CopyTo(scopeAttrs)
			scopeAttrs.PutStr("__scope.name__", sm.Scope().Name())
			scopeAttrs.PutStr("__scope.version__", sm.Scope().Version())
			scopeAttrs.PutStr("__scope.schema_url__", sm.SchemaUrl())
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					c.processGauge(batch, metric, resAttrs, scopeAttrs)
				case pmetric.MetricTypeSum:
					c.processSum(batch, metric, resAttrs, scopeAttrs)
				case pmetric.MetricTypeHistogram:
					c.processHistogram(batch, metric, resAttrs, scopeAttrs)
				case pmetric.MetricTypeSummary:
					c.processSummary(batch, metric, resAttrs, scopeAttrs)
				case pmetric.MetricTypeExponentialHistogram:
					c.processExponentialHistogram(batch, metric, resAttrs, scopeAttrs)
				case pmetric.MetricTypeEmpty:
					c.logger.Warn("metric type is set to empty", zap.String("metric_name", metric.Name()), zap.String("metric_type", metric.Type().String()))
				default:
					c.logger.Warn("unknown metric type", zap.String("metric_name", metric.Name()), zap.String("metric_type", metric.Type().String()))
				}
			}
		}
	}
	c.processMetricsDuration.Record(
		context.Background(),
		float64(time.Since(start).Milliseconds()),
	)
	return batch
}

func (c *clickhouseMetricsExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	c.wg.Add(1)
	defer c.wg.Done()
	return c.writeBatch(ctx, c.prepareBatch(md))
}

func (c *clickhouseMetricsExporter) writeBatch(ctx context.Context, batch *writeBatch) error {

	writeTimeSeries := func(ctx context.Context, timeSeries []ts) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.TimeSeriesTable),
					attribute.String("exporter", pipeline.SignalMetrics.String()),
				),
			)
		}()

		if len(timeSeries) == 0 {
			return nil
		}
		statement, err := c.conn.PrepareBatch(ctx, c.timeSeriesSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		for _, ts := range timeSeries {
			roundedUnixMilli := ts.unixMilli / 3600000 * 3600000
			cacheKey := makeCacheKey(ts.fingerprint, uint64(roundedUnixMilli))
			if item := c.cache.Get(cacheKey); item != nil {
				if value := item.Value(); value {
					continue
				}
			}
			err = statement.Append(
				ts.env,
				ts.temporality.String(),
				ts.metricName,
				ts.description,
				ts.unit,
				ts.typ.String(),
				ts.isMonotonic,
				ts.fingerprint,
				roundedUnixMilli,
				ts.labels,
				ts.attrs,
				ts.scopeAttrs,
				ts.resourceAttrs,
			)
			if err != nil {
				return err
			}
			c.cache.Set(cacheKey, true, ttlcache.DefaultTTL)
		}
		return statement.Send()
	}

	if err := writeTimeSeries(ctx, batch.ts); err != nil {
		return err
	}

	writeSamples := func(ctx context.Context, samples []sample) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.SamplesTable),
					attribute.String("exporter", pipeline.SignalMetrics.String()),
				),
			)
		}()

		if len(samples) == 0 {
			return nil
		}
		statement, err := c.conn.PrepareBatch(ctx, c.samplesSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		for _, sample := range samples {
			err = statement.Append(
				sample.env,
				sample.temporality.String(),
				sample.metricName,
				sample.fingerprint,
				sample.unixMilli,
				sample.value,
			)
			if err != nil {
				return err
			}
		}
		return statement.Send()
	}

	if err := writeSamples(ctx, batch.samples); err != nil {
		return err
	}

	writeExpHist := func(ctx context.Context, expHist []exponentialHistogramSample) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.ExpHistTable),
					attribute.String("exporter", pipeline.SignalMetrics.String()),
				),
			)
		}()

		if len(expHist) == 0 {
			return nil
		}
		statement, err := c.conn.PrepareBatch(ctx, c.expHistSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		for _, expHist := range expHist {
			err = statement.Append(
				expHist.env,
				expHist.temporality.String(),
				expHist.metricName,
				expHist.fingerprint,
				expHist.unixMilli,
				expHist.count,
				expHist.sum,
				expHist.min,
				expHist.max,
				expHist.sketch,
			)
			if err != nil {
				return err
			}
		}
		return statement.Send()
	}

	if err := writeExpHist(ctx, batch.expHist); err != nil {
		return err
	}

	writeMetadata := func(ctx context.Context, metadata []metadata) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.MetadataTable),
					attribute.String("exporter", pipeline.SignalMetrics.String()),
				),
			)
		}()

		if len(metadata) == 0 {
			return nil
		}
		statement, err := c.conn.PrepareBatch(ctx, c.metadataSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		for _, meta := range metadata {
			err = statement.Append(
				meta.metricName,
				meta.temporality.String(),
				meta.description,
				meta.unit,
				meta.typ.String(),
				meta.isMonotonic,
				meta.attrName,
				meta.attrType,
				meta.attrDatatype,
				meta.attrStringValue,
				time.Now().UnixMilli(),
				time.Now().UnixMilli(),
			)
			if err != nil {
				return err
			}
		}
		return statement.Send()
	}

	if err := writeMetadata(ctx, batch.metadata); err != nil {
		// we don't need to return an error here because the metadata is not critical to the operation of the exporter
		// and we don't want to cause the exporter to fail if it is not able to write metadata for some reason
		// if there were a generic error, it would have been returned in the other write functions
		c.logger.Error("error writing metadata", zap.Error(err))
	}

	return nil
}
