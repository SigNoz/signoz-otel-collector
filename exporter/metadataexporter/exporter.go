package metadataexporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
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

const (
	sixHoursInMs = 3600000 * 6
)

type tagValueCountFromDB struct {
	tagDataType         string
	stringTagValueCount uint64
	numberValueCount    uint64
}

type metadataExporter struct {
	cfg Config
	set exporter.Settings

	conn             driver.Conn
	fingerprintCache *ttlcache.Cache[string, bool]
	countCache       *ttlcache.Cache[string, uint64]

	tracesTracker  *ValueTracker
	metricsTracker *ValueTracker
	logsTracker    *ValueTracker

	logTagValueCountFromDB         map[string]tagValueCountFromDB
	logTagValueCountFromDBLock     sync.RWMutex
	logTagValueCountCtx            context.Context
	logTagValueCountCtxCancel      context.CancelFunc
	tracesTagValueCountFromDB      map[string]tagValueCountFromDB
	tracesTagValueCountFromDBLock  sync.RWMutex
	tracesTagValueCountCtx         context.Context
	tracesTagValueCountCtxCancel   context.CancelFunc
	metricsTagValueCountFromDB     map[string]tagValueCountFromDB
	metricsTagValueCountFromDBLock sync.RWMutex
	metricsTagValueCountCtx        context.Context
	metricsTagValueCountCtxCancel  context.CancelFunc

	alwaysIncludeTracesAttributes  map[string]struct{}
	alwaysIncludeLogsAttributes    map[string]struct{}
	alwaysIncludeMetricsAttributes map[string]struct{}
}

func flattenJSONToStringMap(data map[string]any) map[string]string {
	result := make(map[string]string)
	for key, value := range data {
		switch v := value.(type) {
		case string:
			result[key] = v
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
	return result
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

	tracesTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Traces.MaxKeys),
		int(cfg.MaxDistinctValues.Traces.MaxStringDistinctValues*2),
		45*time.Minute,
	)
	metricsTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Metrics.MaxKeys),
		int(cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues*2),
		45*time.Minute,
	)
	logsTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Logs.MaxKeys),
		int(cfg.MaxDistinctValues.Logs.MaxStringDistinctValues*2),
		45*time.Minute,
	)

	logTagValueCountCtx, logTagValueCountCtxCancel := context.WithCancel(context.Background())
	tracesTagValueCountCtx, tracesTagValueCountCtxCancel := context.WithCancel(context.Background())
	metricsTagValueCountCtx, metricsTagValueCountCtxCancel := context.WithCancel(context.Background())

	alwaysIncludeTracesAttributes := make(map[string]struct{})
	alwaysIncludeLogsAttributes := make(map[string]struct{})
	alwaysIncludeMetricsAttributes := make(map[string]struct{})

	for _, attr := range cfg.AlwaysIncludeAttributes.Traces {
		alwaysIncludeTracesAttributes[attr] = struct{}{}
	}
	for _, attr := range cfg.AlwaysIncludeAttributes.Logs {
		alwaysIncludeLogsAttributes[attr] = struct{}{}
	}
	for _, attr := range cfg.AlwaysIncludeAttributes.Metrics {
		alwaysIncludeMetricsAttributes[attr] = struct{}{}
	}
	return &metadataExporter{
		cfg:                            cfg,
		set:                            set,
		conn:                           conn,
		fingerprintCache:               fingerprintCache,
		countCache:                     countCache,
		tracesTracker:                  tracesTracker,
		metricsTracker:                 metricsTracker,
		logsTracker:                    logsTracker,
		logTagValueCountFromDB:         make(map[string]tagValueCountFromDB),
		logTagValueCountFromDBLock:     sync.RWMutex{},
		logTagValueCountCtx:            logTagValueCountCtx,
		logTagValueCountCtxCancel:      logTagValueCountCtxCancel,
		tracesTagValueCountFromDB:      make(map[string]tagValueCountFromDB),
		tracesTagValueCountFromDBLock:  sync.RWMutex{},
		tracesTagValueCountCtx:         tracesTagValueCountCtx,
		tracesTagValueCountCtxCancel:   tracesTagValueCountCtxCancel,
		metricsTagValueCountFromDB:     make(map[string]tagValueCountFromDB),
		metricsTagValueCountFromDBLock: sync.RWMutex{},
		metricsTagValueCountCtx:        metricsTagValueCountCtx,
		metricsTagValueCountCtxCancel:  metricsTagValueCountCtxCancel,
		alwaysIncludeTracesAttributes:  alwaysIncludeTracesAttributes,
		alwaysIncludeLogsAttributes:    alwaysIncludeLogsAttributes,
		alwaysIncludeMetricsAttributes: alwaysIncludeMetricsAttributes,
	}, nil
}

func (e *metadataExporter) Start(ctx context.Context, host component.Host) error {
	e.set.Logger.Info("starting metadata exporter")
	go e.periodicallyUpdateLogTagValueCountFromDB()
	go e.periodicallyUpdateTracesTagValueCountFromDB()
	return nil
}

func (e *metadataExporter) Shutdown(ctx context.Context) error {
	e.set.Logger.Info("shutting down metadata exporter")
	e.tracesTracker.Close()
	e.metricsTracker.Close()
	e.logsTracker.Close()
	e.logTagValueCountCtxCancel()
	e.tracesTagValueCountCtxCancel()
	e.metricsTagValueCountCtxCancel()
	return nil
}

func (e *metadataExporter) periodicallyUpdateLogTagValueCountFromDB() {
	e.set.Logger.Info("starting to periodically update log tag value count from DB")
	// Call the function immediately
	e.updateLogTagValueCountFromDB(e.logTagValueCountCtx)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.updateLogTagValueCountFromDB(e.logTagValueCountCtx)
		}
	}
}

func (e *metadataExporter) updateLogTagValueCountFromDB(ctx context.Context) {
	e.set.Logger.Info("updating log tag value count from DB")
	existingKeys := []string{}
	for k := range e.logTagValueCountFromDB {
		existingKeys = append(existingKeys, k)
	}

	query := `
	select
		tag_key,
		tag_data_type,
		countDistinct(string_value) as string_value_count,
		countDistinct(number_value) as number_value_count
	FROM signoz_logs.distributed_tag_attributes_v2
	group by tag_key, tag_data_type
	order by number_value_count desc, string_value_count desc, tag_key
	limit 1 by tag_key, tag_data_type`

	rows, err := e.conn.Query(ctx, query, existingKeys)
	if err != nil {
		e.set.Logger.Error("failed to query log tag value count from DB", zap.Error(err))
		return
	}
	defer rows.Close()

	e.logTagValueCountFromDBLock.Lock()
	defer e.logTagValueCountFromDBLock.Unlock()

	e.set.Logger.Info("reading log tag value count from DB", zap.Any("query", query))

	for rows.Next() {
		var tagKey string
		var tagDataType string
		var stringTagValueCount uint64
		var numberValueCount uint64
		if err := rows.Scan(&tagKey, &tagDataType, &stringTagValueCount, &numberValueCount); err != nil {
			e.set.Logger.Error("failed to scan log tag value count from DB", zap.Error(err))
			continue
		}
		e.set.Logger.Info("read log tag value count from DB",
			zap.String("tagKey", tagKey),
			zap.String("tagDataType", tagDataType),
			zap.Uint64("stringTagValueCount", stringTagValueCount),
			zap.Uint64("numberValueCount", numberValueCount),
		)
		e.logTagValueCountFromDB[tagKey] = tagValueCountFromDB{
			tagDataType:         tagDataType,
			stringTagValueCount: stringTagValueCount,
			numberValueCount:    numberValueCount,
		}
	}
	e.set.Logger.Info("updated log tag value count from DB", zap.Any("counts", e.logTagValueCountFromDB))
}

func (e *metadataExporter) periodicallyUpdateTracesTagValueCountFromDB() {
	e.set.Logger.Info("starting to periodically update traces tag value count from DB")
	// Call the function immediately
	e.updateTracesTagValueCountFromDB(e.tracesTagValueCountCtx)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.updateTracesTagValueCountFromDB(e.tracesTagValueCountCtx)
		}
	}
}

func (e *metadataExporter) updateTracesTagValueCountFromDB(ctx context.Context) {
	e.set.Logger.Info("updating traces tag value count from DB")

	existingKeys := []string{}
	for k := range e.tracesTagValueCountFromDB {
		existingKeys = append(existingKeys, k)
	}

	query := `
	select
		tag_key,
		tag_data_type,
		countDistinct(string_value) as string_value_count,
		countDistinct(number_value) as number_value_count
	FROM signoz_traces.distributed_tag_attributes_v2
	group by tag_key, tag_data_type
	order by number_value_count desc, string_value_count desc, tag_key
	limit 1 by tag_key, tag_data_type`

	rows, err := e.conn.Query(ctx, query, existingKeys)
	if err != nil {
		e.set.Logger.Error("failed to query traces tag value count from DB", zap.Error(err))
		return
	}
	defer rows.Close()

	e.tracesTagValueCountFromDBLock.Lock()
	defer e.tracesTagValueCountFromDBLock.Unlock()

	e.set.Logger.Info("reading traces tag value count from DB", zap.Any("query", query))

	for rows.Next() {
		var tagKey string
		var tagDataType string
		var stringTagValueCount uint64
		var numberValueCount uint64
		if err := rows.Scan(&tagKey, &tagDataType, &stringTagValueCount, &numberValueCount); err != nil {
			e.set.Logger.Error("failed to scan traces tag value count from DB", zap.Error(err))
			continue
		}
		e.set.Logger.Info("read traces tag value count from DB",
			zap.String("tagKey", tagKey),
			zap.String("tagDataType", tagDataType),
			zap.Uint64("stringTagValueCount", stringTagValueCount),
			zap.Uint64("numberValueCount", numberValueCount),
		)

		e.tracesTagValueCountFromDB[tagKey] = tagValueCountFromDB{
			tagDataType:         tagDataType,
			stringTagValueCount: stringTagValueCount,
			numberValueCount:    numberValueCount,
		}
	}
	e.set.Logger.Info("updated traces tag value count from DB", zap.Any("counts", e.tracesTagValueCountFromDB))
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
	typ := e.getType(key, datasource)
	switch datasource {
	case pipeline.SignalTraces.String():
		if _, ok := e.alwaysIncludeTracesAttributes[key]; ok {
			return false
		}
		cnt = e.tracesTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		switch typ {
		case "string":
			if e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues) {
				return true
			}
		case "float64":
			if e.cfg.MaxDistinctValues.Traces.MaxFloat64DistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Traces.MaxFloat64DistinctValues) {
				return true
			}
		case "int64":
			if e.cfg.MaxDistinctValues.Traces.MaxInt64DistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Traces.MaxInt64DistinctValues) {
				return true
			}
		}
	case pipeline.SignalMetrics.String():
		if _, ok := e.alwaysIncludeMetricsAttributes[key]; ok {
			return false
		}
		cnt = e.metricsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		switch typ {
		case "string":
			if e.cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues) {
				return true
			}
		case "float64":
			if e.cfg.MaxDistinctValues.Metrics.MaxFloat64DistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Metrics.MaxFloat64DistinctValues) {
				return true
			}
		}
	case pipeline.SignalLogs.String():
		if _, ok := e.alwaysIncludeLogsAttributes[key]; ok {
			return false
		}
		cnt = e.logsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		switch typ {
		case "string":
			if e.cfg.MaxDistinctValues.Logs.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Logs.MaxStringDistinctValues) {
				return true
			}
		case "int64":
			if e.cfg.MaxDistinctValues.Logs.MaxInt64DistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Logs.MaxInt64DistinctValues) {
				return true
			}
		case "float64":
			if e.cfg.MaxDistinctValues.Logs.MaxFloat64DistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Logs.MaxFloat64DistinctValues) {
				return true
			}
		}
	}
	return false
}

func (e *metadataExporter) filterAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	filteredAttrs := make(map[string]any)
	skippedAttrs := []string{}
	nonSkipAttrs := []string{}
	for k, v := range attrs {
		uvtKey := makeUVTKey(k, datasource)
		vStr := fmt.Sprintf("%v", v)
		if len(vStr) > int(e.cfg.MaxDistinctValues.Traces.MaxStringLength) || len(vStr) == 0 {
			skippedAttrs = append(skippedAttrs, k)
			continue
		}
		// Add to UVT first
		e.addToUVT(ctx, uvtKey, vStr, datasource)

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

func (e *metadataExporter) getType(key string, datasource string) string {
	switch datasource {
	case pipeline.SignalTraces.String():
		return e.tracesTagValueCountFromDB[key].tagDataType
	case pipeline.SignalLogs.String():
		return e.logTagValueCountFromDB[key].tagDataType
	case pipeline.SignalMetrics.String():
		return e.metricsTagValueCountFromDB[key].tagDataType
	}
	return "string"
}

func (e *metadataExporter) shouldSkipAttributeFromDB(_ context.Context, key string, datasource string) bool {
	switch datasource {
	case pipeline.SignalTraces.String():
		if _, ok := e.alwaysIncludeTracesAttributes[key]; ok {
			return false
		}
		switch e.tracesTagValueCountFromDB[key].tagDataType {
		case "string":
			return e.tracesTagValueCountFromDB[key].stringTagValueCount > e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues
		case "float64":
			return true
			// return e.tracesTagValueCountFromDB[key].float64TagValueCount > e.cfg.MaxDistinctValues.Traces.MaxFloat64DistinctValues
		}
	case pipeline.SignalLogs.String():
		if _, ok := e.alwaysIncludeLogsAttributes[key]; ok {
			return false
		}
		switch e.logTagValueCountFromDB[key].tagDataType {
		case "string":
			return e.logTagValueCountFromDB[key].stringTagValueCount > e.cfg.MaxDistinctValues.Logs.MaxStringDistinctValues
		case "int64":
			// return e.logTagValueCountFromDB[key].int64TagValueCount > e.cfg.MaxDistinctValues.Logs.MaxInt64DistinctValues
			return true
		case "float64":
			// return e.logTagValueCountFromDB[key].float64TagValueCount > e.cfg.MaxDistinctValues.Logs.MaxFloat64DistinctValues
			return true
		}
	case pipeline.SignalMetrics.String():
		if _, ok := e.alwaysIncludeMetricsAttributes[key]; ok {
			return false
		}
		switch e.metricsTagValueCountFromDB[key].tagDataType {
		case "string":
			return e.metricsTagValueCountFromDB[key].stringTagValueCount > e.cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues
		case "float64":
			return e.metricsTagValueCountFromDB[key].numberValueCount > e.cfg.MaxDistinctValues.Metrics.MaxFloat64DistinctValues
		}
	}
	return false
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	e.tracesTagValueCountFromDBLock.RLock()
	defer e.tracesTagValueCountFromDBLock.RUnlock()

	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	totalSpans := 0
	writtenSpans := 0

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
				totalSpans++
				span := ss.Spans().At(k)
				spanAttrs := make(map[string]any)
				skippedFromDB := []string{}
				span.Attributes().Range(func(k string, v pcommon.Value) bool {
					if e.shouldSkipAttributeFromDB(ctx, k, pipeline.SignalTraces.String()) {
						skippedFromDB = append(skippedFromDB, k)
						return true
					}
					spanAttrs[k] = v.AsRaw()
					return true
				})
				if len(skippedFromDB) > 0 {
					e.set.Logger.Info("skipped attributes from DB", zap.String("datasource", pipeline.SignalTraces.String()), zap.Strings("keys", skippedFromDB))
				}
				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				filteredSpanAttrs := e.filterAttrs(ctx, flattenedSpanAttrs, pipeline.SignalTraces.String())
				spanFingerprint := fingerprint.FingerprintHash(filteredSpanAttrs)
				unixMilli := span.StartTimestamp().AsTime().UnixMilli()
				roundedUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs
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
					flattenJSONToStringMap(flattenedResourceAttrs),
					flattenJSONToStringMap(filteredSpanAttrs),
				)
				writtenSpans++
				if err != nil {
					return err
				}
				e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
			}
		}
	}

	e.set.Logger.Info("pushed traces attributes", zap.Int("total_spans", totalSpans), zap.Int("written_spans", writtenSpans))

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
					metricFingerprint := fingerprint.FingerprintHash(flattenedMetricAttrs)
					unixMilli := time.Now().UnixMilli()
					roundedUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs
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
						flattenJSONToStringMap(flattenedResourceAttrs),
						flattenJSONToStringMap(flattenedMetricAttrs),
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
	e.logTagValueCountFromDBLock.RLock()
	defer e.logTagValueCountFromDBLock.RUnlock()

	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	totalLogRecords := 0
	writtenLogRecords := 0
	writtenLogRecordsFingerprints := []uint64{}

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
				totalLogRecords++
				logRecord := sl.LogRecords().At(k)
				logRecordAttrs := make(map[string]any)
				skippedFromDB := []string{}
				logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
					if e.shouldSkipAttributeFromDB(ctx, k, pipeline.SignalLogs.String()) {
						skippedFromDB = append(skippedFromDB, k)
						return true
					}
					logRecordAttrs[k] = v.AsRaw()
					return true
				})
				if len(skippedFromDB) > 0 {
					e.set.Logger.Info("skipped attributes from DB", zap.String("datasource", pipeline.SignalLogs.String()), zap.Strings("keys", skippedFromDB))
				}
				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				filteredLogRecordAttrs := e.filterAttrs(ctx, flattenedLogRecordAttrs, pipeline.SignalLogs.String())
				logRecordFingerprint := fingerprint.FingerprintHash(filteredLogRecordAttrs)
				unixMilli := logRecord.Timestamp().AsTime().UnixMilli()
				roundedUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs
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
					flattenJSONToStringMap(flattenedResourceAttrs),
					flattenJSONToStringMap(filteredLogRecordAttrs),
				)
				if err != nil {
					return err
				}
				e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
				writtenLogRecords++
				writtenLogRecordsFingerprints = append(writtenLogRecordsFingerprints, logRecordFingerprint)
			}
		}
	}

	e.set.Logger.Info("pushed logs attributes",
		zap.Int("total_log_records", totalLogRecords),
		zap.Int("written_log_records", writtenLogRecords),
		zap.Uint64s("written_log_records_fingerprints", writtenLogRecordsFingerprints),
	)

	return stmt.Send()
}
