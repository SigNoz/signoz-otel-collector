package signozclickhousemetrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.opentelemetry.io/otel/attribute"
	metricapi "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

var (
	countSuffix       = ".count"
	sumSuffix         = ".sum"
	minSuffix         = ".min"
	maxSuffix         = ".max"
	bucketSuffix      = ".bucket"
	quantilesSuffix   = ".quantile"
	samplesSQLTmpl    = "INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, value, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	timeSeriesSQLTmpl = "INSERT INTO %s.%s (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels, attrs, scope_attrs, resource_attrs, __normalized, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	expHistSQLTmpl    = "INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, count, sum, min, max, sketch, flags, inserted_at_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	metadataSQLTmpl   = "INSERT INTO %s.%s (temporality, metric_name, description, unit, type, is_monotonic, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
)

const NanDetectedErrMsg = "NaN detected in data point, skipping entire data point"

type clickhouseMetricsExporter struct {
	cfg           *Config
	logger        *zap.Logger
	meter         metricapi.Meter
	cache         *ttlcache.Cache[string, bool]
	cacheRunning  bool
	conn          clickhouse.Conn
	wg            sync.WaitGroup
	enableExpHist bool

	samplesSQL    string
	timeSeriesSQL string
	expHistSQL    string
	metadataSQL   string

	processMetricsDuration metricapi.Float64Histogram
	exportMetricsDuration  metricapi.Float64Histogram
	settings               exporter.Settings

	usageCollector *usage.UsageCollector
	exporterID     uuid.UUID

	closeChan chan struct{}
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
	flags       uint32
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
	flags       uint32
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
	metricName             string
	temporality            pmetric.AggregationTemporality
	description            string
	unit                   string
	typ                    pmetric.MetricType
	isMonotonic            bool
	attrName               string
	attrType               string
	attrDatatype           pcommon.ValueType
	attrStringValue        string
	firstReportedUnixMilli int64
	lastReportedUnixMilli  int64
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

func WithSettings(settings exporter.Settings) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.settings = settings
		return nil
	}
}

func WithUsageCollector(collector *usage.UsageCollector) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.usageCollector = collector
		return nil
	}
}

func WithExporterID(exporterID uuid.UUID) ExporterOption {
	return func(e *clickhouseMetricsExporter) error {
		e.exporterID = exporterID
		return nil
	}
}

func defaultOptions() []ExporterOption {
	cache := ttlcache.New(
		ttlcache.WithTTL[string, bool](45*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)

	return []ExporterOption{
		WithCache(cache),
		WithLogger(zap.NewNop()),
		WithEnableExpHist(false),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
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

	if chExporter.usageCollector != nil {
		err = chExporter.usageCollector.Start()
		if err != nil {
			return nil, err
		}
		if err := view.Register(MetricPointsCountView, MetricPointsBytesView); err != nil {
			return nil, err
		}
	}
	chExporter.closeChan = make(chan struct{})

	return chExporter, nil
}

func (c *clickhouseMetricsExporter) Start(ctx context.Context, host component.Host) error {
	go c.cache.Start()
	c.cacheRunning = true
	return nil
}

func (c *clickhouseMetricsExporter) Shutdown(ctx context.Context) error {
	if c.cacheRunning {
		c.cache.Stop()
	}
	if c.usageCollector != nil {
		err := c.usageCollector.Stop()
		if err != nil {
			c.logger.Error("failed to stop usage collector", zap.Error(err))
		}
	}
	close(c.closeChan)
	c.wg.Wait()
	return c.conn.Close()
}

// processGauge processes gauge metrics
func (c *clickhouseMetricsExporter) processGauge(batch *batch, metric pmetric.Metric, env string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// gauge metrics do not have a temporality
	temporality := pmetric.AggregationTemporalityUnspecified
	// there is no monotonicity for gauge metrics
	isMonotonic := false

	firstSeenUnixMilli := int64(math.MaxInt64)
	lastSeenUnixMilli := int64(0)

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
		dp := metric.Gauge().DataPoints().At(i)
		unixMilli := dp.Timestamp().AsTime().UnixMilli()

		// Track min/max timestamps for resource/scope metadata
		firstSeenUnixMilli = min(firstSeenUnixMilli, unixMilli)
		lastSeenUnixMilli = max(lastSeenUnixMilli, unixMilli)

		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		if math.IsNaN(value) {
			c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
			continue
		}

		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: fingerprint.HashWithName(name),
			unixMilli:   unixMilli,
			value:       value,
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)
		batch.addTs(&ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	// Add resource/scope metadata AFTER loop with tracked timestamps
	batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
}

// processSum processes sum metrics
func (c *clickhouseMetricsExporter) processSum(batch *batch, metric pmetric.Metric, env string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {

	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// sum metrics have a temporality
	temporality := metric.Sum().AggregationTemporality()
	isMonotonic := metric.Sum().IsMonotonic()

	firstSeenUnixMilli := int64(math.MaxInt64)
	lastSeenUnixMilli := int64(0)

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
		dp := metric.Sum().DataPoints().At(i)
		unixMilli := dp.Timestamp().AsTime().UnixMilli()

		// Track min/max timestamps for resource/scope metadata
		firstSeenUnixMilli = min(firstSeenUnixMilli, unixMilli)
		lastSeenUnixMilli = max(lastSeenUnixMilli, unixMilli)

		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		if math.IsNaN(value) {
			c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
			continue
		}
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: fingerprint.HashWithName(name),
			unixMilli:   unixMilli,
			value:       value,
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)
		batch.addTs(&ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	// Add resource/scope metadata AFTER loop with tracked timestamps
	batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
}

// processHistogram processes histogram metrics
func (c *clickhouseMetricsExporter) processHistogram(b *batch, metric pmetric.Metric, env string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	temporality := metric.Histogram().AggregationTemporality()
	// monotonicity is assumed for histograms
	isMonotonic := true

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	addSample := func(batch *batch, dp pmetric.HistogramDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		sampleTyp := typ
		sampleTemporality := temporality
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
			sampleTyp = pmetric.MetricTypeSum
		case sumSuffix:
			value = dp.Sum()
			sampleTyp = pmetric.MetricTypeSum
		case minSuffix:
			value = dp.Min()
			sampleTyp = pmetric.MetricTypeGauge
			sampleTemporality = pmetric.AggregationTemporalityUnspecified
		case maxSuffix:
			value = dp.Max()
			sampleTyp = pmetric.MetricTypeGauge
			sampleTemporality = pmetric.AggregationTemporalityUnspecified
		}
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": sampleTemporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: sampleTemporality,
			metricName:  name + suffix,
			fingerprint: fingerprint.HashWithName(name + suffix),
			unixMilli:   unixMilli,
			value:       value,
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name+suffix, desc, unit, sampleTyp, sampleTemporality, isMonotonic, fingerprint, unixMilli, unixMilli)

		batch.addTs(&ts{
			env:           env,
			temporality:   sampleTemporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           sampleTyp,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name + suffix),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	addBucketSample := func(batch *batch, dp pmetric.HistogramDataPoint, suffix string) {
		var cumulativeCount uint64
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		pointAttrs := dp.Attributes()

		for i := 0; i < dp.ExplicitBounds().Len() && i < dp.BucketCounts().Len(); i++ {
			bound := dp.ExplicitBounds().At(i)
			cumulativeCount += dp.BucketCounts().At(i)
			boundStr := strconv.FormatFloat(bound, 'f', -1, 64)

			fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), pointAttrs, map[string]string{
				"le":              boundStr,
				"__temporality__": temporality.String(),
			})
			fingerprintMap := fingerprint.AttributesAsMap()

			batch.addSample(&sample{
				env:         env,
				temporality: temporality,
				metricName:  name + suffix,
				fingerprint: fingerprint.HashWithName(name + suffix),
				unixMilli:   unixMilli,
				value:       float64(cumulativeCount),
				flags:       uint32(dp.Flags()),
			})
			batch.addMetadata(name+suffix, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)

			batch.addTs(&ts{
				env:           env,
				temporality:   temporality,
				metricName:    name + suffix,
				description:   desc,
				unit:          unit,
				typ:           typ,
				isMonotonic:   isMonotonic,
				fingerprint:   fingerprint.HashWithName(name + suffix),
				unixMilli:     unixMilli,
				labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
				attrs:         fingerprintMap,
				scopeAttrs:    scopeFingerprintMap,
				resourceAttrs: resourceFingerprintMap,
			})
		}

		// add le=+Inf sample
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), pointAttrs, map[string]string{
			"le":              "+Inf",
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: fingerprint.HashWithName(name + suffix),
			unixMilli:   unixMilli,
			value:       float64(dp.Count()),
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name+suffix, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)
		batch.addTs(&ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name + suffix),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	firstSeenUnixMilli := int64(math.MaxInt64)
	lastSeenUnixMilli := int64(0)

	for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
		dp := metric.Histogram().DataPoints().At(i)
		unixMilli := dp.Timestamp().AsTime().UnixMilli()

		// Track min/max timestamps for resource/scope metadata
		firstSeenUnixMilli = min(firstSeenUnixMilli, unixMilli)
		lastSeenUnixMilli = max(lastSeenUnixMilli, unixMilli)

		// we need to create five samples for each histogram dp
		// 1. count
		// 2. sum
		// 3. min
		// 4. max
		// 5. bucket counts
		// Check if any of the key float fields are NaN.
		if math.IsNaN(dp.Sum()) || math.IsNaN(dp.Min()) || math.IsNaN(dp.Max()) {
			c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
			continue
		}
		addSample(b, dp, countSuffix)
		if dp.HasSum() {
			addSample(b, dp, sumSuffix)
		}
		if dp.HasMin() {
			addSample(b, dp, minSuffix)
		}
		if dp.HasMax() {
			addSample(b, dp, maxSuffix)
		}
		addBucketSample(b, dp, bucketSuffix)
	}

	// Add resource/scope metadata for all suffixes after processing all datapoints
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+minSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+minSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+maxSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+maxSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+bucketSuffix, desc, unit, typ, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+bucketSuffix, desc, unit, typ, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
}

func (c *clickhouseMetricsExporter) processSummary(b *batch, metric pmetric.Metric, env string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// summaries are cumulative by default
	temporality := pmetric.AggregationTemporalityCumulative
	// monotonicity is assumed for summaries
	isMonotonic := true

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	addSample := func(batch *batch, dp pmetric.SummaryDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		sampleTyp := typ
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
			sampleTyp = pmetric.MetricTypeSum
		case sumSuffix:
			value = dp.Sum()
			sampleTyp = pmetric.MetricTypeSum
		}
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: temporality,
			metricName:  name + suffix,
			fingerprint: fingerprint.HashWithName(name + suffix),
			unixMilli:   unixMilli,
			value:       value,
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name+suffix, desc, unit, sampleTyp, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)

		batch.addTs(&ts{
			env:           env,
			temporality:   temporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           sampleTyp,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name + suffix),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	addQuantileSample := func(batch *batch, dp pmetric.SummaryDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		for i := 0; i < dp.QuantileValues().Len(); i++ {
			quantile := dp.QuantileValues().At(i)
			quantileStr := strconv.FormatFloat(quantile.Quantile(), 'f', -1, 64)
			quantileValue := quantile.Value()

			fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
				"quantile":        quantileStr,
				"__temporality__": temporality.String(),
			})
			fingerprintMap := fingerprint.AttributesAsMap()
			batch.addSample(&sample{
				env:         env,
				temporality: temporality,
				metricName:  name + suffix,
				fingerprint: fingerprint.HashWithName(name + suffix),
				unixMilli:   unixMilli,
				value:       quantileValue,
				flags:       uint32(dp.Flags()),
			})
			batch.addMetadata(name+suffix, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)
			batch.addTs(&ts{
				env:           env,
				temporality:   temporality,
				metricName:    name + suffix,
				description:   desc,
				unit:          unit,
				typ:           typ,
				isMonotonic:   isMonotonic,
				fingerprint:   fingerprint.HashWithName(name + suffix),
				unixMilli:     unixMilli,
				labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
				attrs:         fingerprintMap,
				scopeAttrs:    scopeFingerprintMap,
				resourceAttrs: resourceFingerprintMap,
			})
		}
	}

	firstSeenUnixMilli := int64(math.MaxInt64)
	lastSeenUnixMilli := int64(0)

	for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
		dp := metric.Summary().DataPoints().At(i)
		unixMilli := dp.Timestamp().AsTime().UnixMilli()

		// Track min/max timestamps for resource/scope metadata
		firstSeenUnixMilli = min(firstSeenUnixMilli, unixMilli)
		lastSeenUnixMilli = max(lastSeenUnixMilli, unixMilli)

		skip := false

		if math.IsNaN(dp.Sum()) {
			c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
			skip = true
		}

		// Check for NaN in quantiles
		quantiles := dp.QuantileValues()
		for j := 0; j < quantiles.Len(); j++ {
			q := quantiles.At(j)
			if math.IsNaN(q.Value()) {
				c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
				skip = true
				break
			}
		}

		if skip {
			continue
		}
		addSample(b, dp, countSuffix)
		addSample(b, dp, sumSuffix)
		addQuantileSample(b, dp, quantilesSuffix)
	}

	// Add resource/scope metadata for all suffixes after processing all datapoints
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+quantilesSuffix, desc, unit, typ, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+quantilesSuffix, desc, unit, typ, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
}

func (c *clickhouseMetricsExporter) processExponentialHistogram(b *batch, metric pmetric.Metric, env string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {
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

	isMonotonic := true

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	addSample := func(batch *batch, dp pmetric.ExponentialHistogramDataPoint, suffix string) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		sampleTyp := typ
		sampleTemporality := temporality
		var value float64
		switch suffix {
		case countSuffix:
			value = float64(dp.Count())
			sampleTyp = pmetric.MetricTypeSum
		case sumSuffix:
			value = dp.Sum()
			sampleTyp = pmetric.MetricTypeSum
		case minSuffix:
			value = dp.Min()
			sampleTyp = pmetric.MetricTypeGauge
			sampleTemporality = pmetric.AggregationTemporalityUnspecified
		case maxSuffix:
			value = dp.Max()
			sampleTyp = pmetric.MetricTypeGauge
			sampleTemporality = pmetric.AggregationTemporalityUnspecified
		}
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			env:         env,
			temporality: sampleTemporality,
			metricName:  name + suffix,
			fingerprint: fingerprint.HashWithName(name + suffix),
			unixMilli:   unixMilli,
			value:       value,
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name+suffix, desc, unit, sampleTyp, sampleTemporality, isMonotonic, fingerprint, unixMilli, unixMilli)

		batch.addTs(&ts{
			env:           env,
			temporality:   sampleTemporality,
			metricName:    name + suffix,
			description:   desc,
			unit:          unit,
			typ:           sampleTyp,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name + suffix),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name+suffix, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprintMap,
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
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

	addDDSketchSample := func(batch *batch, dp pmetric.ExponentialHistogramDataPoint) {
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		positive := toStore(dp.Positive())
		negative := toStore(dp.Negative())
		gamma := math.Pow(2, math.Pow(2, float64(-dp.Scale())))
		dd := chproto.DD{
			Mapping:        &chproto.IndexMapping{Gamma: gamma},
			PositiveValues: positive,
			NegativeValues: negative,
			ZeroCount:      float64(dp.ZeroCount()),
		}

		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		batch.addExpHist(&exponentialHistogramSample{
			env:         env,
			temporality: temporality,
			metricName:  name,
			fingerprint: fingerprint.HashWithName(name),
			unixMilli:   unixMilli,
			sketch:      dd,
			count:       float64(dp.Count()),
			sum:         dp.Sum(),
			min:         dp.Min(),
			max:         dp.Max(),
			flags:       uint32(dp.Flags()),
		})
		batch.addMetadata(name, desc, unit, typ, temporality, isMonotonic, fingerprint, unixMilli, unixMilli)

		batch.addTs(&ts{
			env:           env,
			temporality:   temporality,
			metricName:    name,
			description:   desc,
			unit:          unit,
			typ:           typ,
			isMonotonic:   isMonotonic,
			fingerprint:   fingerprint.HashWithName(name),
			unixMilli:     unixMilli,
			labels:        pkgfingerprint.NewLabelsAsJSONString(name, fingerprint.AttributesAsMap(), scopeFingerprintMap, resourceFingerprintMap),
			attrs:         fingerprint.AttributesAsMap(),
			scopeAttrs:    scopeFingerprintMap,
			resourceAttrs: resourceFingerprintMap,
		})
	}

	firstSeenUnixMilli := int64(math.MaxInt64)
	lastSeenUnixMilli := int64(0)

	for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
		dp := metric.ExponentialHistogram().DataPoints().At(i)
		unixMilli := dp.Timestamp().AsTime().UnixMilli()

		// Track min/max timestamps for resource/scope metadata
		firstSeenUnixMilli = min(firstSeenUnixMilli, unixMilli)
		lastSeenUnixMilli = max(lastSeenUnixMilli, unixMilli)

		// we need to create five samples for each exponential histogram dp
		// 1. count
		// 2. sum
		// 3. min
		// 4. max
		// 5. ddsketch
		if math.IsNaN(dp.Sum()) || math.IsNaN(dp.Min()) || math.IsNaN(dp.Max()) {
			c.logger.Debug(NanDetectedErrMsg, zap.String("metric_name", name))
			continue
		}
		addSample(b, dp, countSuffix)
		if dp.HasSum() {
			addSample(b, dp, sumSuffix)
		}
		if dp.HasMin() {
			addSample(b, dp, minSuffix)
		}
		if dp.HasMax() {
			addSample(b, dp, maxSuffix)
		}
		addDDSketchSample(b, dp)
	}

	// Add resource/scope metadata for all suffixes after processing all datapoints
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+countSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+sumSuffix, desc, unit, pmetric.MetricTypeSum, temporality, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+minSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+minSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)

	b.addMetadata(name+maxSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, resourceFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
	b.addMetadata(name+maxSuffix, desc, unit, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, isMonotonic, scopeFingerprint, firstSeenUnixMilli, lastSeenUnixMilli)
}

func (c *clickhouseMetricsExporter) prepareBatch(ctx context.Context, md pmetric.Metrics) *batch {
	batch := newBatch(c.logger)
	start := time.Now()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceFingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.ResourceFingerprintType, pkgfingerprint.InitialOffset, rm.Resource().Attributes(), map[string]string{})

		env := ""
		if de, ok := rm.Resource().Attributes().Get(semconv.AttributeDeploymentEnvironment); ok {
			env = de.AsString()
		}

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeFingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.ScopeFingerprintType, resourceFingerprint.Hash(), sm.Scope().Attributes(), map[string]string{
				"__scope.name__":       sm.Scope().Name(),
				"__scope.version__":    sm.Scope().Version(),
				"__scope.schema_url__": sm.SchemaUrl(),
			})

			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					c.processGauge(batch, metric, env, resourceFingerprint, scopeFingerprint)
				case pmetric.MetricTypeSum:
					c.processSum(batch, metric, env, resourceFingerprint, scopeFingerprint)
				case pmetric.MetricTypeHistogram:
					c.processHistogram(batch, metric, env, resourceFingerprint, scopeFingerprint)
				case pmetric.MetricTypeSummary:
					c.processSummary(batch, metric, env, resourceFingerprint, scopeFingerprint)
				case pmetric.MetricTypeExponentialHistogram:
					c.processExponentialHistogram(batch, metric, env, resourceFingerprint, scopeFingerprint)
				case pmetric.MetricTypeEmpty:
					c.logger.Warn("metric type is set to empty", zap.String("metric_name", metric.Name()), zap.String("metric_type", metric.Type().String()))
				default:
					c.logger.Warn("unknown metric type", zap.String("metric_name", metric.Name()), zap.String("metric_type", metric.Type().String()))
				}
			}
		}
	}
	c.processMetricsDuration.Record(
		ctx,
		float64(time.Since(start).Milliseconds()),
		metricapi.WithAttributes(
			attribute.String("exporter", c.settings.ID.String()),
		),
	)
	return batch
}

func (c *clickhouseMetricsExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.closeChan:
		return errors.New("shutdown has been called")
	default:
		return c.writeBatch(ctx, c.prepareBatch(ctx, md))
	}
}

func (c *clickhouseMetricsExporter) writeBatch(ctx context.Context, batch *batch) error {
	writeTimeSeries := func(ctx context.Context, timeSeries []*ts) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.TimeSeriesTable),
					attribute.String("exporter", c.settings.ID.String()),
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
		defer statement.Close()

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
				false,
				time.Now().UnixMilli(),
			)
			if err != nil {
				return err
			}
			c.cache.Set(cacheKey, true, ttlcache.DefaultTTL)
		}
		return statement.Send()
	}

	writeSamples := func(ctx context.Context, samples []*sample) error {
		start := time.Now()
		metrics := map[string]usage.Metric{}

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.SamplesTable),
					attribute.String("exporter", c.settings.ID.String()),
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
		defer statement.Close()

		for _, sample := range samples {
			err = statement.Append(
				sample.env,
				sample.temporality.String(),
				sample.metricName,
				sample.fingerprint,
				sample.unixMilli,
				sample.value,
				sample.flags,
				time.Now().UnixMilli(),
			)
			if err != nil {
				return err
			}
			collectUsage := true
			if strings.HasPrefix(sample.metricName, "signoz") || strings.HasPrefix(sample.metricName, "chi") || strings.HasPrefix(sample.metricName, "otelcol") {
				collectUsage = false
			}

			if collectUsage {
				usage.AddMetric(metrics, "default", 1, 0)
			}
		}
		for k, v := range metrics {
			err = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, c.exporterID.String())}, ExporterSigNozSentMetricPoints.M(int64(v.Count)), ExporterSigNozSentMetricPointsBytes.M(int64(v.Size)))
			if err != nil {
				c.logger.Error("error recording usage metric", zap.Error(err))
			}
		}
		return statement.Send()
	}

	writeExpHist := func(ctx context.Context, expHist []*exponentialHistogramSample) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.ExpHistTable),
					attribute.String("exporter", c.settings.ID.String()),
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
		defer statement.Close()

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
				expHist.flags,
				time.Now().UnixMilli(),
			)
			if err != nil {
				return err
			}
		}
		return statement.Send()
	}

	writeMetadata := func(ctx context.Context, metadata []*metadata) error {
		start := time.Now()

		defer func() {
			c.exportMetricsDuration.Record(
				ctx,
				float64(time.Since(start).Milliseconds()),
				metricapi.WithAttributes(
					attribute.String("table", c.cfg.MetadataTable),
					attribute.String("exporter", c.settings.ID.String()),
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
		defer statement.Close()

		for _, meta := range metadata {
			err = statement.Append(
				meta.temporality.String(),
				meta.metricName,
				meta.description,
				meta.unit,
				meta.typ.String(),
				meta.isMonotonic,
				meta.attrName,
				meta.attrType,
				meta.attrDatatype,
				meta.attrStringValue,
				meta.firstReportedUnixMilli,
				meta.lastReportedUnixMilli,
			)
			if err != nil {
				return err
			}
		}
		return statement.Send()
	}

	// Send all statements in parallel
	errC := make(chan error, 4)

	go func() {
		errC <- writeTimeSeries(ctx, batch.ts)
	}()

	go func() {
		errC <- writeSamples(ctx, batch.samples)
	}()

	go func() {
		errC <- writeExpHist(ctx, batch.expHist)
	}()

	go func() {
		if err := writeMetadata(ctx, batch.metadata); err != nil {
			// we don't need to return an error here because the metadata is not critical to the operation of the exporter
			// and we don't want to cause the exporter to fail if it is not able to write metadata for some reason
			// if there were a generic error, it would have been returned in the other write functions
			c.logger.Error("error writing metadata", zap.Error(err))
		}

		errC <- nil
	}()

	var errs []error
	for i := 0; i < 4; i++ {
		if err := <-errC; err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func makeCacheKey(a, b uint64) string {
	var builder strings.Builder
	builder.Grow(40) // Max length: 20 digits for each uint64 + 1 for the colon

	// Convert and write the first uint64
	builder.WriteString(strconv.FormatUint(a, 10))

	// Write the separator
	builder.WriteByte(':')

	// Convert and write the second uint64
	builder.WriteString(strconv.FormatUint(b, 10))

	return builder.String()
}
