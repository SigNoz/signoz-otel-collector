package metadataexporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
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
	sixHours                       = 6 * time.Hour                      // window size for attributes aggregation
	sixHoursInMs                   = int64(sixHours / time.Millisecond) // window size in ms
	maxValuesInCache               = 3_000_000                          // max number of values for fingerprint cache
	valuTrackerKeysTTL             = 45 * time.Minute                   // ttl for keys in value tracker
	fetchKeysDistinctCountInterval = 15 * time.Minute
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

	tracesTracker *ValueTracker

	tracesTagValueCountFromDB    atomic.Pointer[map[string]tagValueCountFromDB]
	tracesTagValueCountCtx       context.Context
	tracesTagValueCountCtxCancel context.CancelFunc

	alwaysIncludeTracesAttributes map[string]struct{}
}

func flattenJSONToStringMap(data map[string]any) map[string]string {
	res := make(map[string]string, len(data))
	for k, v := range data {
		res[k] = fmt.Sprint(v)
	}
	return res
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
		ttlcache.WithTTL[string, bool](sixHours),
		ttlcache.WithDisableTouchOnHit[string, bool](),        // don't update the ttl when the item is accessed
		ttlcache.WithCapacity[string, bool](maxValuesInCache), // max 3M items in the cache
	)
	go fingerprintCache.Start()

	tracesTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Traces.MaxKeys),
		int(cfg.MaxDistinctValues.Traces.MaxStringDistinctValues),
		valuTrackerKeysTTL, // if a key is not seen in 45 mins, it is removed from the tracker
	)

	tracesTagValueCountCtx, tracesTagValueCountCtxCancel := context.WithCancel(context.Background())

	alwaysIncludeTraces := make(map[string]struct{}, len(cfg.AlwaysIncludeAttributes.Traces))
	for _, attr := range cfg.AlwaysIncludeAttributes.Traces {
		alwaysIncludeTraces[attr] = struct{}{}
	}

	// Initialize atomic pointers to empty maps.
	initMap := func() *map[string]tagValueCountFromDB {
		m := make(map[string]tagValueCountFromDB)
		return &m
	}

	e := &metadataExporter{
		cfg:                           cfg,
		set:                           set,
		conn:                          conn,
		fingerprintCache:              fingerprintCache,
		tracesTracker:                 tracesTracker,
		tracesTagValueCountCtx:        tracesTagValueCountCtx,
		tracesTagValueCountCtxCancel:  tracesTagValueCountCtxCancel,
		alwaysIncludeTracesAttributes: alwaysIncludeTraces,
	}

	e.tracesTagValueCountFromDB.Store(initMap())

	return e, nil
}

func (e *metadataExporter) Start(_ context.Context, host component.Host) error {
	e.set.Logger.Info("starting metadata exporter")

	go e.periodicallyUpdateTagValueCountFromDB(
		e.tracesTagValueCountCtx,
		&updateParams{
			logger: e.set.Logger,
			conn:   e.conn,
			query: `SELECT tag_key, tag_data_type, countDistinct(string_value) as string_value_count, countDistinct(number_value) as number_value_count
						 FROM signoz_traces.distributed_tag_attributes_v2
						 GROUP BY tag_key, tag_data_type
						 ORDER BY number_value_count DESC, string_value_count DESC, tag_key
						 LIMIT 1 BY tag_key, tag_data_type`,
			storeFunc:  e.storeTracesTagValues,
			signalName: pipeline.SignalTraces.String(),
			interval:   fetchKeysDistinctCountInterval,
		},
	)

	return nil
}

func (e *metadataExporter) Shutdown(_ context.Context) error {
	e.set.Logger.Info("shutting down metadata exporter")
	e.tracesTracker.Close()
	e.tracesTagValueCountCtxCancel()
	return nil
}

type updateParams struct {
	logger     *zap.Logger
	conn       driver.Conn
	query      string
	storeFunc  func(map[string]tagValueCountFromDB)
	signalName string
	interval   time.Duration
}

func (e *metadataExporter) periodicallyUpdateTagValueCountFromDB(ctx context.Context, params *updateParams) {
	params.logger.Info("starting periodic update for tag values", zap.String("signal", params.signalName))
	e.updateTagValueCountFromDB(ctx, params)

	ticker := time.NewTicker(params.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.updateTagValueCountFromDB(ctx, params)
		}
	}
}

func (e *metadataExporter) updateTagValueCountFromDB(ctx context.Context, p *updateParams) {
	p.logger.Info("updating tag value count from DB", zap.String("signal", p.signalName))
	rows, err := p.conn.Query(ctx, p.query)
	if err != nil {
		p.logger.Error("failed to query tag value counts", zap.String("signal", p.signalName), zap.Error(err))
		return
	}
	defer rows.Close()

	newMap := make(map[string]tagValueCountFromDB)
	for rows.Next() {
		var tagKey, tagDataType string
		var stringCount, numberCount uint64

		if err := rows.Scan(&tagKey, &tagDataType, &stringCount, &numberCount); err != nil {
			p.logger.Error("failed to scan tag value count", zap.String("signal", p.signalName), zap.Error(err))
			continue
		}

		newMap[tagKey] = tagValueCountFromDB{
			tagDataType:         tagDataType,
			stringTagValueCount: stringCount,
			numberValueCount:    numberCount,
		}
	}

	p.storeFunc(newMap)
	p.logger.Info("updated tag value count from DB", zap.String("signal", p.signalName), zap.Int("countSize", len(newMap)))
}

func (e *metadataExporter) storeTracesTagValues(newValues map[string]tagValueCountFromDB) {
	e.storeTagValuesAtomic(&e.tracesTagValueCountFromDB, newValues)
}

func (e *metadataExporter) storeTagValuesAtomic(target *atomic.Pointer[map[string]tagValueCountFromDB], newValues map[string]tagValueCountFromDB) {
	target.Store(&newValues)
}

// Helper to get tag type for given datasource
func (e *metadataExporter) getType(key, datasource string) string {
	var m *map[string]tagValueCountFromDB
	switch datasource {
	case pipeline.SignalTraces.String():
		m = e.tracesTagValueCountFromDB.Load()
	default:
		return "string"
	}

	val, ok := (*m)[key]
	if !ok {
		return "string"
	}
	return val.tagDataType
}

func (e *metadataExporter) shouldSkipAttributeFromDB(_ context.Context, key, datasource string) bool {
	var m *map[string]tagValueCountFromDB
	var alwaysInclude map[string]struct{}
	var cfgMax LimitsConfig

	switch datasource {
	case pipeline.SignalTraces.String():
		m = e.tracesTagValueCountFromDB.Load()
		alwaysInclude = e.alwaysIncludeTracesAttributes
		cfgMax = e.cfg.MaxDistinctValues.Traces
	default:
		return false
	}

	if _, ok := alwaysInclude[key]; ok {
		return false
	}
	val, ok := (*m)[key]
	if !ok {
		return false
	}

	switch val.tagDataType {
	case "string":
		return val.stringTagValueCount > cfgMax.MaxStringDistinctValues
	case "float64", "int64":
		return true
	}
	return false
}

func makeUVTKey(key, datasource string) string {
	builder := strings.Builder{}
	builder.Grow(len(key) + 1 + len(datasource))
	builder.WriteString(key)
	builder.WriteByte(':')
	builder.WriteString(datasource)
	return builder.String()
}

func (e *metadataExporter) addToUVT(_ context.Context, key, value, datasource string) {
	switch datasource {
	case pipeline.SignalTraces.String():
		e.tracesTracker.AddValue(key, value)
	}
}

func (e *metadataExporter) shouldSkipAttributeUVT(_ context.Context, key, datasource string) bool {
	typ := e.getType(key, datasource)
	var cnt int

	switch datasource {
	case pipeline.SignalTraces.String():
		if _, ok := e.alwaysIncludeTracesAttributes[key]; ok {
			return false
		}
		cnt = e.tracesTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		if typ == "string" && e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues) {
			return true
		}
		if typ == "float64" || typ == "int64" {
			return true
		}
	}
	return false
}

func (e *metadataExporter) filterAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	filtered := make(map[string]any, len(attrs))
	var skippedAttrs, nonSkipAttrs []string

	maxLen := int(e.cfg.MaxDistinctValues.Traces.MaxStringLength)
	for k, v := range attrs {
		vStr := fmt.Sprint(v)
		if len(vStr) == 0 || len(vStr) > maxLen {
			skippedAttrs = append(skippedAttrs, k)
			continue
		}
		e.addToUVT(ctx, makeUVTKey(k, datasource), vStr, datasource)
		if !e.shouldSkipAttributeUVT(ctx, k, datasource) {
			filtered[k] = v
			nonSkipAttrs = append(nonSkipAttrs, k)
		} else {
			skippedAttrs = append(skippedAttrs, k)
		}
	}

	e.set.Logger.Debug("filtered attributes",
		zap.String("datasource", datasource),
		zap.Int("skipped_count", len(skippedAttrs)),
		zap.Strings("skipped_attributes", skippedAttrs),
		zap.Strings("non_skipped_attributes", nonSkipAttrs),
	)
	return filtered
}

func makeFingerprintCacheKey(a, b uint64, datasource string) string {
	builder := strings.Builder{}
	builder.Grow(40 + len(datasource))
	builder.WriteString(strconv.FormatUint(a, 10))
	builder.WriteByte(':')
	builder.WriteString(strconv.FormatUint(b, 10))
	builder.WriteByte(':')
	builder.WriteString(datasource)
	return builder.String()
}

func (e *metadataExporter) writeToStmt(_ context.Context, stmt driver.Batch, ds pipeline.Signal, resourceFingerprint, fprint uint64, rAttrs, filtered map[string]any, roundedSixHrsUnixMilli int64) (bool, error) {
	cacheKey := makeFingerprintCacheKey(fprint, uint64(roundedSixHrsUnixMilli), ds.String())
	if item := e.fingerprintCache.Get(cacheKey); item != nil && item.Value() {
		return true, nil
	}

	if err := stmt.Append(
		roundedSixHrsUnixMilli,
		ds,
		resourceFingerprint,
		fprint,
		flattenJSONToStringMap(rAttrs),
		flattenJSONToStringMap(filtered),
	); err != nil {
		return false, err
	}

	e.fingerprintCache.Set(cacheKey, true, ttlcache.DefaultTTL)
	return false, nil
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	stmt, err := e.conn.PrepareBatch(ctx, "INSERT INTO signoz_metadata.distributed_attributes_metadata", driver.WithReleaseConnection())
	if err != nil {
		return err
	}

	totalSpans := 0
	skippedSpans := 0

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resourceAttrs := make(map[string]any, rs.Resource().Attributes().Len())
		rs.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if e.shouldSkipAttributeFromDB(ctx, k, pipeline.SignalTraces.String()) {
				return true
			}
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				totalSpans++
				span := spans.At(k)
				spanAttrs := make(map[string]any, span.Attributes().Len())
				var skippedFromDB []string

				span.Attributes().Range(func(attrKey string, v pcommon.Value) bool {
					if e.shouldSkipAttributeFromDB(ctx, attrKey, pipeline.SignalTraces.String()) {
						skippedFromDB = append(skippedFromDB, attrKey)
						return true
					}
					spanAttrs[attrKey] = v.AsRaw()
					return true
				})

				if len(skippedFromDB) > 0 {
					e.set.Logger.Debug("skipped attributes from DB (traces)", zap.Strings("keys", skippedFromDB))
				}

				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				filteredSpanAttrs := e.filterAttrs(ctx, flattenedSpanAttrs, pipeline.SignalTraces.String())
				spanFingerprint := fingerprint.FingerprintHash(filteredSpanAttrs)

				unixMilli := span.StartTimestamp().AsTime().UnixMilli()
				roundedSixHrsUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs

				skipped, err := e.writeToStmt(ctx, stmt, pipeline.SignalTraces, resourceFingerprint, spanFingerprint, flattenedResourceAttrs, filteredSpanAttrs, roundedSixHrsUnixMilli)
				if err != nil {
					e.set.Logger.Error("failed to write to stmt", zap.Error(err))
				}
				if skipped {
					skippedSpans++
				}
			}
		}
	}

	e.set.Logger.Debug("pushed traces attributes", zap.Int("total_spans", totalSpans), zap.Int("skipped_spans", skippedSpans))

	if err := stmt.Send(); err != nil {
		e.set.Logger.Error("failed to send stmt", zap.Error(err))
	}
	return nil
}

func (e *metadataExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (e *metadataExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
