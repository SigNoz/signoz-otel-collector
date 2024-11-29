package metadataexporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type metadataExporter struct {
	cfg Config
	set exporter.Settings

	conn             driver.Conn
	fingerprintCache *ttlcache.Cache[string, bool]
	countCache       *ttlcache.Cache[string, uint64]

	tracesTracker  *ValueTracker
	metricsTracker *ValueTracker
	logsTracker    *ValueTracker
}

func newMetadataExporter(cfg Config, set exporter.Settings) (*metadataExporter, error) {
	opts, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}
	fingerprintCache := ttlcache.New[string, bool](
		ttlcache.WithTTL[string, bool](45*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, bool](),
	)
	go fingerprintCache.Start()
	countCache := ttlcache.New[string, uint64](
		ttlcache.WithTTL[string, uint64](15*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, uint64](),
	)
	go countCache.Start()

	tracesTracker := NewValueTracker(int(cfg.MaxDistinctValues*2), int(cfg.MaxDistinctValues*2), 45*time.Minute)
	metricsTracker := NewValueTracker(int(cfg.MaxDistinctValues*2), int(cfg.MaxDistinctValues*2), 45*time.Minute)
	logsTracker := NewValueTracker(int(cfg.MaxDistinctValues*2), int(cfg.MaxDistinctValues*2), 45*time.Minute)

	return &metadataExporter{
		cfg:              cfg,
		set:              set,
		conn:             conn,
		fingerprintCache: fingerprintCache,
		countCache:       countCache,
		tracesTracker:    tracesTracker,
		metricsTracker:   metricsTracker,
		logsTracker:      logsTracker,
	}, nil
}

func (e *metadataExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *metadataExporter) Shutdown(ctx context.Context) error {
	e.tracesTracker.Close()
	e.metricsTracker.Close()
	e.logsTracker.Close()
	return nil
}

func makeFingerprintCacheKey(a, b uint64, datasource string) string {
	// Pre-allocate a builder with an estimated capacity
	var builder strings.Builder
	builder.Grow(64) // Max length: 20 digits for each uint64 + 1 for the colon + ~10 for the datasource + buffer

	// Convert and write the first uint64
	builder.WriteString(strconv.FormatUint(a, 10))

	// Write the separator
	builder.WriteByte(':')

	// Convert and write the second uint64
	builder.WriteString(strconv.FormatUint(b, 10))

	// Write the separator
	builder.WriteByte(':')

	// Write the datasource
	builder.WriteString(datasource)

	return builder.String()
}

func makeUVTKey(key string, datasource string) string {
	var builder strings.Builder
	builder.Grow(256)
	builder.WriteString(key)
	builder.WriteByte(':')
	builder.WriteString(datasource)
	return builder.String()
}

func (e *metadataExporter) addToUVT(_ context.Context, key string, value string, datasource string) {
	switch datasource {
	case pipeline.SignalTraces.String():
		e.tracesTracker.AddValue(key, value)
	case pipeline.SignalMetrics.String():
		e.metricsTracker.AddValue(key, value)
	case pipeline.SignalLogs.String():
		e.logsTracker.AddValue(key, value)
	}
}

func (e *metadataExporter) shouldSkipAttributeUVT(_ context.Context, key string, datasource string) bool {
	var cnt int
	switch datasource {
	case pipeline.SignalTraces.String():
		cnt = e.tracesTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
	case pipeline.SignalMetrics.String():
		cnt = e.metricsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
	case pipeline.SignalLogs.String():
		cnt = e.logsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
	}
	if cnt > 100 {
		e.set.Logger.Info("unique value count", zap.String("key", key), zap.String("datasource", datasource), zap.Int("value", cnt))
	}
	return cnt > int(e.cfg.MaxDistinctValues)
}

func (e *metadataExporter) filterAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	filteredAttrs := make(map[string]any)
	skippedAttrs := []string{}
	nonSkipAttrs := []string{}
	for k, v := range attrs {
		uvtKey := makeUVTKey(k, datasource)
		// Add to UVT first
		e.addToUVT(ctx, uvtKey, fmt.Sprintf("%v", v), datasource)

		// Check local UVT count
		if !e.shouldSkipAttributeUVT(ctx, k, datasource) {
			filteredAttrs[k] = v
			nonSkipAttrs = append(nonSkipAttrs, k)
		} else {
			skippedAttrs = append(skippedAttrs, k)
		}
	}
	e.set.Logger.Info("filtered attributes",
		zap.String("datasource", datasource),
		zap.Int("count", len(skippedAttrs)),
		zap.Strings("skipped_attributes", skippedAttrs),
		zap.Strings("non_skipped_attributes", nonSkipAttrs),
	)
	return filteredAttrs
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {

	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resourceAttrs := make(map[string]any)
		rs.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			resourceAttrs[k] = v.AsRaw()
			return true
		})

		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				spanAttrs := make(map[string]any)
				span.Attributes().Range(func(k string, v pcommon.Value) bool {
					spanAttrs[k] = v.AsRaw()
					return true
				})
				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				filteredSpanAttrs := e.filterAttrs(ctx, flattenedSpanAttrs, pipeline.SignalTraces.String())
				spanFingerprint := fingerprint.FingerprintHash(filteredSpanAttrs)
				unixMilli := span.StartTimestamp().AsTime().UnixMilli()
				roundedUnixMilli := unixMilli / 3600000 * 3600000
				cacheKey := makeFingerprintCacheKey(spanFingerprint, uint64(roundedUnixMilli), pipeline.SignalTraces.String())
				if item := e.fingerprintCache.Get(cacheKey); item != nil {
					if value := item.Value(); value {
						continue
					}
				}
				err = stmt.Append(
					roundedUnixMilli,
					pipeline.SignalTraces,
					resourceFingerprint,
					spanFingerprint,
					flatten.FlattenJSONToStringMap(flattenedResourceAttrs),
					flatten.FlattenJSONToStringMap(filteredSpanAttrs),
				)
				if err != nil {
					return err
				}
				e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
			}
		}
	}

	return stmt.Send()
}

func (e *metadataExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		resourceAttrs := make(map[string]any)
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		scopeMetrics := rm.ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			sm := scopeMetrics.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				var pAttrs []pcommon.Map
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					for l := 0; l < metric.Gauge().DataPoints().Len(); l++ {
						pAttrs = append(pAttrs, metric.Gauge().DataPoints().At(l).Attributes())
					}
				case pmetric.MetricTypeSum:
					for l := 0; l < metric.Sum().DataPoints().Len(); l++ {
						pAttrs = append(pAttrs, metric.Sum().DataPoints().At(l).Attributes())
					}
				case pmetric.MetricTypeHistogram:
					for l := 0; l < metric.Histogram().DataPoints().Len(); l++ {
						pAttrs = append(pAttrs, metric.Histogram().DataPoints().At(l).Attributes())
					}
				case pmetric.MetricTypeExponentialHistogram:
					for l := 0; l < metric.ExponentialHistogram().DataPoints().Len(); l++ {
						pAttrs = append(pAttrs, metric.ExponentialHistogram().DataPoints().At(l).Attributes())
					}
				case pmetric.MetricTypeSummary:
					for l := 0; l < metric.Summary().DataPoints().Len(); l++ {
						pAttrs = append(pAttrs, metric.Summary().DataPoints().At(l).Attributes())
					}
				}

				for _, pAttr := range pAttrs {
					metricAttrs := make(map[string]any)
					pAttr.Range(func(k string, v pcommon.Value) bool {
						metricAttrs[k] = v.AsRaw()
						return true
					})
					flattenedMetricAttrs := flatten.FlattenJSON(metricAttrs, "")
					filteredMetricAttrs := e.filterAttrs(ctx, flattenedMetricAttrs, pipeline.SignalMetrics.String())
					metricFingerprint := fingerprint.FingerprintHash(filteredMetricAttrs)
					unixMilli := time.Now().UnixMilli()
					roundedUnixMilli := unixMilli / 3600000 * 3600000
					cacheKey := makeFingerprintCacheKey(uint64(roundedUnixMilli), metricFingerprint, pipeline.SignalMetrics.String())
					if item := e.fingerprintCache.Get(cacheKey); item != nil {
						if value := item.Value(); value {
							continue
						}
					}
					err = stmt.Append(
						roundedUnixMilli,
						pipeline.SignalMetrics,
						resourceFingerprint,
						metricFingerprint,
						flatten.FlattenJSONToStringMap(flattenedResourceAttrs),
						flatten.FlattenJSONToStringMap(filteredMetricAttrs),
					)
					if err != nil {
						return err
					}
					e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
				}
			}
		}
	}

	return stmt.Send()
}

func (e *metadataExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resourceAttrs := make(map[string]any)
		rl.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				logRecord := sl.LogRecords().At(k)
				logRecordAttrs := make(map[string]any)
				logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
					logRecordAttrs[k] = v.AsRaw()
					return true
				})
				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				filteredLogRecordAttrs := e.filterAttrs(ctx, flattenedLogRecordAttrs, pipeline.SignalLogs.String())
				logRecordFingerprint := fingerprint.FingerprintHash(filteredLogRecordAttrs)
				unixMilli := logRecord.Timestamp().AsTime().UnixMilli()
				roundedUnixMilli := unixMilli / 3600000 * 3600000
				cacheKey := makeFingerprintCacheKey(uint64(roundedUnixMilli), logRecordFingerprint, pipeline.SignalLogs.String())
				if item := e.fingerprintCache.Get(cacheKey); item != nil {
					if value := item.Value(); value {
						continue
					}
				}
				err = stmt.Append(
					roundedUnixMilli,
					pipeline.SignalLogs,
					resourceFingerprint,
					logRecordFingerprint,
					flatten.FlattenJSONToStringMap(flattenedResourceAttrs),
					flatten.FlattenJSONToStringMap(filteredLogRecordAttrs),
				)
				if err != nil {
					return err
				}
				e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
			}
		}
	}

	return stmt.Send()
}
