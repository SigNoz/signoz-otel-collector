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

	return &metadataExporter{cfg: cfg, set: set, conn: conn, fingerprintCache: fingerprintCache, countCache: countCache}, nil
}

func (e *metadataExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *metadataExporter) Shutdown(ctx context.Context) error {
	return nil
}

// shouldSkipAttribute checks if the distinct values for the attribute exceed the limit
// and if so, skips the attribute
func (e *metadataExporter) shouldSkipAttribute(ctx context.Context, key string, datasource string) bool {
	cacheKey := makeCountCacheKey(key, datasource)
	if item := e.countCache.Get(cacheKey); item != nil {
		var value uint64
		if value = item.Value(); value > e.cfg.MaxDistinctValues {
			e.set.Logger.Debug("skipping attribute as distinct values exceed limit",
				zap.String("key", key), zap.String("datasource", datasource), zap.Uint64("value", value))
			return true
		} else {
			e.set.Logger.Debug("distinct values count is within limit",
				zap.String("key", key), zap.String("datasource", datasource), zap.Uint64("value", value))
			return false
		}
	}

	sql := fmt.Sprintf(`
	SELECT countDistinct(attributes['%s'])
	FROM signoz_metadata.distributed_attributes_metadata
	WHERE data_source = '%s' AND mapContains(attributes, '%s') AND attributes['%s'] IS NOT NULL
	AND rounded_unix_milli > toUnixTimestamp(now() - INTERVAL 6 HOUR) * 1000
	`, key, datasource, key, key)

	e.set.Logger.Debug("fetching distinct values count",
		zap.String("key", key), zap.String("datasource", datasource))

	var cnt uint64
	if err := e.conn.QueryRow(ctx, sql).Scan(&cnt); err == nil {
		e.countCache.Set(cacheKey, cnt, ttlcache.DefaultTTL)
		e.set.Logger.Debug("fetched distinct values count",
			zap.String("key", key), zap.String("datasource", datasource), zap.Uint64("value", cnt))
		return cnt > e.cfg.MaxDistinctValues
	}
	return false
}

func makeFingerprintCacheKey(a, b uint64, datasource string) string {
	// Pre-allocate a builder with an estimated capacity
	var builder strings.Builder
	builder.Grow(40) // Max length: 20 digits for each uint64 + 1 for the colon

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

func makeCountCacheKey(key string, datasource string) string {
	var builder strings.Builder
	builder.Grow(128)
	builder.WriteString(key)
	builder.WriteByte(':')
	builder.WriteString(datasource)
	return builder.String()
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {

	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer func() {
		stmt.Abort()
	}()

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
					if e.shouldSkipAttribute(ctx, k, pipeline.SignalTraces.String()) {
						return true
					}
					spanAttrs[k] = v.AsRaw()
					return true
				})
				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				spanFingerprint := fingerprint.FingerprintHash(flattenedSpanAttrs)
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
					flatten.FlattenJSONToStringMap(flattenedSpanAttrs),
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
	defer func() {
		stmt.Abort()
	}()

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
						if e.shouldSkipAttribute(ctx, k, pipeline.SignalMetrics.String()) {
							return true
						}
						metricAttrs[k] = v.AsRaw()
						return true
					})
					flattenedMetricAttrs := flatten.FlattenJSON(metricAttrs, "")
					metricFingerprint := fingerprint.FingerprintHash(flattenedMetricAttrs)
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
						flatten.FlattenJSONToStringMap(flattenedMetricAttrs),
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
	defer func() {
		stmt.Abort()
	}()

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
					if e.shouldSkipAttribute(ctx, k, pipeline.SignalLogs.String()) {
						return true
					}
					logRecordAttrs[k] = v.AsRaw()
					return true
				})
				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				logRecordFingerprint := fingerprint.FingerprintHash(flattenedLogRecordAttrs)
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
					flatten.FlattenJSONToStringMap(flattenedLogRecordAttrs),
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
