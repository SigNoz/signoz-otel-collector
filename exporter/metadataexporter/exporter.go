package metadataexporter

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	kash "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/cache"
)

const (
	sixHours           = 6 * time.Hour                      // window size for attributes aggregation
	sixHoursInMs       = int64(sixHours / time.Millisecond) // window size in ms
	valuTrackerKeysTTL = 45 * time.Minute                   // ttl for keys in value tracker
	insertStmtQuery    = "INSERT INTO signoz_metadata.distributed_attributes_metadata"
)

type tagValueCountFromDB struct {
	tagDataType         string
	stringTagValueCount uint64
	numberValueCount    uint64
}

type metadataExporter struct {
	cfg Config
	set exporter.Settings

	conn     driver.Conn
	keyCache kash.KeyCache

	tracesTracker  *ValueTracker
	metricsTracker *ValueTracker
	logsTracker    *ValueTracker

	logTagValueCountFromDB    atomic.Pointer[map[string]tagValueCountFromDB]
	logTagValueCountCtx       context.Context
	logTagValueCountCtxCancel context.CancelFunc

	tracesTagValueCountFromDB    atomic.Pointer[map[string]tagValueCountFromDB]
	tracesTagValueCountCtx       context.Context
	tracesTagValueCountCtxCancel context.CancelFunc

	metricsTagValueCountFromDB    atomic.Pointer[map[string]tagValueCountFromDB]
	metricsTagValueCountCtx       context.Context
	metricsTagValueCountCtxCancel context.CancelFunc

	alwaysIncludeTracesAttributes  map[string]struct{}
	alwaysIncludeLogsAttributes    map[string]struct{}
	alwaysIncludeMetricsAttributes map[string]struct{}
}

type writeToStatementBatchRecord struct {
	resourceFingerprint    uint64
	fprint                 uint64
	rAttrs                 map[string]any
	attrs                  map[string]any
	roundedSixHrsUnixMilli int64
}

func flattenJSONToStringMap(data map[string]any) map[string]string {
	res := make(map[string]string, len(data))
	for k, v := range data {
		switch v := v.(type) {
		case string:
			res[k] = v
		}
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

	set.Logger.Info("cache provider", zap.String("provider", string(cfg.Cache.Provider)))
	var keyCache kash.KeyCache
	var cacheErr error

	if cfg.Cache.Provider == CacheProviderRedis {
		keyCache, cacheErr = kash.NewRedisKeyCache(kash.RedisKeyCacheOptions{
			Addr:     cfg.Cache.Redis.Addr,
			Username: cfg.Cache.Redis.Username,
			Password: cfg.Cache.Redis.Password,
			DB:       cfg.Cache.Redis.DB,
			TenantID: cfg.TenantID,
			Logger:   set.Logger,

			TracesTTL:  sixHours,
			MetricsTTL: sixHours,
			LogsTTL:    sixHours,

			MaxTracesResourceFp:              cfg.Cache.Traces.MaxResources,
			MaxMetricsResourceFp:             cfg.Cache.Metrics.MaxResources,
			MaxLogsResourceFp:                cfg.Cache.Logs.MaxResources,
			MaxTracesCardinalityPerResource:  cfg.Cache.Traces.MaxCardinalityPerResource,
			MaxMetricsCardinalityPerResource: cfg.Cache.Metrics.MaxCardinalityPerResource,
			MaxLogsCardinalityPerResource:    cfg.Cache.Logs.MaxCardinalityPerResource,
			TracesMaxTotalCardinality:        cfg.Cache.Traces.MaxTotalCardinality,
			MetricsMaxTotalCardinality:       cfg.Cache.Metrics.MaxTotalCardinality,
			LogsMaxTotalCardinality:          cfg.Cache.Logs.MaxTotalCardinality,
			Debug:                            cfg.Cache.Debug,
		})
	} else {
		keyCache, cacheErr = kash.NewInMemoryKeyCache(kash.InMemoryKeyCacheOptions{
			MaxTracesResourceFp:              cfg.Cache.Traces.MaxResources,
			MaxMetricsResourceFp:             cfg.Cache.Metrics.MaxResources,
			MaxLogsResourceFp:                cfg.Cache.Logs.MaxResources,
			MaxTracesCardinalityPerResource:  cfg.Cache.Traces.MaxCardinalityPerResource,
			MaxMetricsCardinalityPerResource: cfg.Cache.Metrics.MaxCardinalityPerResource,
			MaxLogsCardinalityPerResource:    cfg.Cache.Logs.MaxCardinalityPerResource,
			TracesFingerprintCacheTTL:        sixHours,
			MetricsFingerprintCacheTTL:       sixHours,
			LogsFingerprintCacheTTL:          sixHours,
			TenantID:                         cfg.TenantID,
			Logger:                           set.Logger,
			TracesMaxTotalCardinality:        cfg.Cache.Traces.MaxTotalCardinality,
			MetricsMaxTotalCardinality:       cfg.Cache.Metrics.MaxTotalCardinality,
			LogsMaxTotalCardinality:          cfg.Cache.Logs.MaxTotalCardinality,
			Debug:                            cfg.Cache.Debug,
		})
	}

	if cacheErr != nil {
		return nil, errors.Wrap(cacheErr, "failed to create key cache")
	}

	tracesTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Traces.MaxKeys),
		int(cfg.MaxDistinctValues.Traces.MaxStringDistinctValues),
		valuTrackerKeysTTL,
	)
	metricsTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Metrics.MaxKeys),
		int(cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues),
		valuTrackerKeysTTL,
	)
	logsTracker := NewValueTracker(
		int(cfg.MaxDistinctValues.Logs.MaxKeys),
		int(cfg.MaxDistinctValues.Logs.MaxStringDistinctValues),
		valuTrackerKeysTTL,
	)

	logTagValueCountCtx, logTagValueCountCtxCancel := context.WithCancel(context.Background())
	tracesTagValueCountCtx, tracesTagValueCountCtxCancel := context.WithCancel(context.Background())
	metricsTagValueCountCtx, metricsTagValueCountCtxCancel := context.WithCancel(context.Background())

	alwaysIncludeTraces := make(map[string]struct{}, len(cfg.AlwaysIncludeAttributes.Traces))
	for _, attr := range cfg.AlwaysIncludeAttributes.Traces {
		alwaysIncludeTraces[attr] = struct{}{}
	}

	alwaysIncludeLogs := make(map[string]struct{}, len(cfg.AlwaysIncludeAttributes.Logs))
	for _, attr := range cfg.AlwaysIncludeAttributes.Logs {
		alwaysIncludeLogs[attr] = struct{}{}
	}

	alwaysIncludeMetrics := make(map[string]struct{}, len(cfg.AlwaysIncludeAttributes.Metrics))
	for _, attr := range cfg.AlwaysIncludeAttributes.Metrics {
		alwaysIncludeMetrics[attr] = struct{}{}
	}

	// Initialize atomic pointers to empty maps.
	initMap := func() *map[string]tagValueCountFromDB {
		m := make(map[string]tagValueCountFromDB)
		return &m
	}

	e := &metadataExporter{
		cfg:  cfg,
		set:  set,
		conn: conn,

		keyCache: keyCache,

		tracesTracker:                  tracesTracker,
		metricsTracker:                 metricsTracker,
		logsTracker:                    logsTracker,
		logTagValueCountCtx:            logTagValueCountCtx,
		logTagValueCountCtxCancel:      logTagValueCountCtxCancel,
		tracesTagValueCountCtx:         tracesTagValueCountCtx,
		tracesTagValueCountCtxCancel:   tracesTagValueCountCtxCancel,
		metricsTagValueCountCtx:        metricsTagValueCountCtx,
		metricsTagValueCountCtxCancel:  metricsTagValueCountCtxCancel,
		alwaysIncludeTracesAttributes:  alwaysIncludeTraces,
		alwaysIncludeLogsAttributes:    alwaysIncludeLogs,
		alwaysIncludeMetricsAttributes: alwaysIncludeMetrics,
	}

	e.logTagValueCountFromDB.Store(initMap())
	e.tracesTagValueCountFromDB.Store(initMap())
	e.metricsTagValueCountFromDB.Store(initMap())

	return e, nil
}

func (e *metadataExporter) Start(_ context.Context, host component.Host) error {
	if !e.cfg.Enabled {
		return nil
	}
	e.set.Logger.Info("starting metadata exporter")

	go e.periodicallyUpdateTagValueCountFromDB(
		e.logTagValueCountCtx,
		&updateParams{
			logger: e.set.Logger,
			conn:   e.conn,
			query: `SELECT tag_key, tag_data_type, countDistinct(string_value) as string_value_count, countDistinct(number_value) as number_value_count
						 FROM signoz_logs.distributed_tag_attributes_v2
						 WHERE unix_milli >= toUnixTimestamp(now() - INTERVAL 6 HOUR) * 1000
						 GROUP BY tag_key, tag_data_type
						 ORDER BY number_value_count DESC, string_value_count DESC, tag_key
						 LIMIT 1 BY tag_key, tag_data_type
						 SETTINGS max_threads = 2`,
			storeFunc:  e.storeLogTagValues,
			signalName: pipeline.SignalLogs.String(),
			interval:   e.cfg.MaxDistinctValues.Logs.FetchInterval,
		},
	)

	go e.periodicallyUpdateTagValueCountFromDB(
		e.tracesTagValueCountCtx,
		&updateParams{
			logger: e.set.Logger,
			conn:   e.conn,
			query: `SELECT tag_key, tag_data_type, countDistinct(string_value) as string_value_count, countDistinct(number_value) as number_value_count
						 FROM signoz_traces.distributed_tag_attributes_v2
						 WHERE unix_milli >= toUnixTimestamp(now() - INTERVAL 6 HOUR) * 1000
						 GROUP BY tag_key, tag_data_type
						 ORDER BY number_value_count DESC, string_value_count DESC, tag_key
						 LIMIT 1 BY tag_key, tag_data_type
						 SETTINGS max_threads = 2`,
			storeFunc:  e.storeTracesTagValues,
			signalName: pipeline.SignalTraces.String(),
			interval:   e.cfg.MaxDistinctValues.Traces.FetchInterval,
		},
	)

	return nil
}

func (e *metadataExporter) Shutdown(ctx context.Context) error {
	e.set.Logger.Info("shutting down metadata exporter")

	e.logTagValueCountCtxCancel()
	e.tracesTagValueCountCtxCancel()
	e.metricsTagValueCountCtxCancel()

	e.tracesTracker.Close()
	e.metricsTracker.Close()
	e.logsTracker.Close()

	if e.keyCache != nil {
		if err := e.keyCache.Close(ctx); err != nil {
			e.set.Logger.Error("failed to close key cache", zap.Error(err))
		}
	}

	if e.conn != nil {
		if err := e.conn.Close(); err != nil {
			e.set.Logger.Error("failed to close clickhouse connection", zap.Error(err))
		}
	}

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
			e.keyCache.Debug(ctx)
			e.updateTagValueCountFromDB(ctx, params)
		}
	}
}

func (e *metadataExporter) updateTagValueCountFromDB(ctx context.Context, p *updateParams) {
	p.logger.Debug("updating tag value count from DB", zap.String("signal", p.signalName))
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
	p.logger.Debug("updated tag value count from DB", zap.String("signal", p.signalName), zap.Int("countSize", len(newMap)))
}

func (e *metadataExporter) storeLogTagValues(newValues map[string]tagValueCountFromDB) {
	e.storeTagValuesAtomic(&e.logTagValueCountFromDB, newValues)
}

func (e *metadataExporter) storeTracesTagValues(newValues map[string]tagValueCountFromDB) {
	e.storeTagValuesAtomic(&e.tracesTagValueCountFromDB, newValues)
}

func (e *metadataExporter) storeTagValuesAtomic(target *atomic.Pointer[map[string]tagValueCountFromDB], newValues map[string]tagValueCountFromDB) {
	target.Store(&newValues)
}

// getType gets the tag type for a given key and datasource
func (e *metadataExporter) getType(key, datasource string) string {
	var m *map[string]tagValueCountFromDB
	switch datasource {
	case pipeline.SignalTraces.String():
		m = e.tracesTagValueCountFromDB.Load()
	case pipeline.SignalLogs.String():
		m = e.logTagValueCountFromDB.Load()
	case pipeline.SignalMetrics.String():
		m = e.metricsTagValueCountFromDB.Load()
	default:
		return "string"
	}

	val, ok := (*m)[key]
	if !ok {
		return "string"
	}
	return val.tagDataType
}

// shouldSkipAttributeFromDB checks if an attribute should be skipped based on the tag value count from DB
func (e *metadataExporter) shouldSkipAttributeFromDB(_ context.Context, key, datasource string) bool {
	var m *map[string]tagValueCountFromDB
	var alwaysInclude map[string]struct{}
	var cfgMax LimitsConfig

	switch datasource {
	case pipeline.SignalTraces.String():
		m = e.tracesTagValueCountFromDB.Load()
		alwaysInclude = e.alwaysIncludeTracesAttributes
		cfgMax = e.cfg.MaxDistinctValues.Traces
	case pipeline.SignalLogs.String():
		m = e.logTagValueCountFromDB.Load()
		alwaysInclude = e.alwaysIncludeLogsAttributes
		cfgMax = e.cfg.MaxDistinctValues.Logs
	case pipeline.SignalMetrics.String():
		m = e.metricsTagValueCountFromDB.Load()
		alwaysInclude = e.alwaysIncludeMetricsAttributes
		cfgMax = e.cfg.MaxDistinctValues.Metrics
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

// addToUVT adds a value to the unique value tracker
func (e *metadataExporter) addToUVT(_ context.Context, key, value, datasource string) {
	switch datasource {
	case pipeline.SignalTraces.String():
		e.tracesTracker.AddValue(key, value)
	case pipeline.SignalMetrics.String():
		e.metricsTracker.AddValue(key, value)
	case pipeline.SignalLogs.String():
		e.logsTracker.AddValue(key, value)
	}
}

// shouldSkipAttributeUVT checks if an attribute should be skipped based on the unique value tracker
func (e *metadataExporter) shouldSkipAttributeUVT(_ context.Context, key, datasource string) bool {
	typ := e.getType(key, datasource)
	var cnt int

	switch datasource {
	case pipeline.SignalTraces.String():
		if _, ok := e.alwaysIncludeTracesAttributes[key]; ok {
			return false
		}
		cnt = e.tracesTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		if typ == utils.TagDataTypeString.String() && e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Traces.MaxStringDistinctValues) {
			return true
		}
		if typ == utils.TagDataTypeNumber.String() {
			return true
		}

	case pipeline.SignalMetrics.String():
		if _, ok := e.alwaysIncludeMetricsAttributes[key]; ok {
			return false
		}
		cnt = e.metricsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		if typ == utils.TagDataTypeString.String() && e.cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Metrics.MaxStringDistinctValues) {
			return true
		}
		if typ == utils.TagDataTypeNumber.String() {
			return true
		}

	case pipeline.SignalLogs.String():
		if _, ok := e.alwaysIncludeLogsAttributes[key]; ok {
			return false
		}
		cnt = e.logsTracker.GetUniqueValueCount(makeUVTKey(key, datasource))
		if typ == utils.TagDataTypeString.String() && e.cfg.MaxDistinctValues.Logs.MaxStringDistinctValues > 0 && cnt > int(e.cfg.MaxDistinctValues.Logs.MaxStringDistinctValues) {
			return true
		}
		if typ == utils.TagDataTypeNumber.String() {
			return true
		}
	}
	return false
}

func removeDuplicateRecords(records []writeToStatementBatchRecord) []writeToStatementBatchRecord {
	seen := make(map[uint64]map[uint64]writeToStatementBatchRecord)
	for _, rec := range records {
		if _, ok := seen[rec.resourceFingerprint]; !ok {
			seen[rec.resourceFingerprint] = make(map[uint64]writeToStatementBatchRecord)
		}
		seen[rec.resourceFingerprint][rec.fprint] = rec
	}
	uniqueRecords := make([]writeToStatementBatchRecord, 0)
	for _, rec := range seen {
		for _, r := range rec {
			uniqueRecords = append(uniqueRecords, r)
		}
	}
	return uniqueRecords
}

func (e *metadataExporter) writeToStatementBatch(ctx context.Context, stmt driver.Batch, records []writeToStatementBatchRecord, ds pipeline.Signal) (int, error) {

	records = removeDuplicateRecords(records)

	var existsCheckDuration, addAttrsDuration, resourcesLimitCheckDuration, totalCardinalityLimitCheckDuration time.Duration
	resourcesLimitCheckStart := time.Now()
	// check max resources limit
	if e.keyCache.ResourcesLimitExceeded(ctx, ds) {
		e.set.Logger.Debug("resource limit exceeded", zap.String("datasource", ds.String()), zap.Int("records", len(records)))
		return 0, nil
	}
	resourcesLimitCheckDuration = time.Since(resourcesLimitCheckStart)

	totalCardinalityLimitCheckStart := time.Now()
	if e.keyCache.TotalCardinalityLimitExceeded(ctx, ds) {
		e.set.Logger.Debug("total cardinality limit exceeded", zap.String("datasource", ds.String()), zap.Int("records", len(records)))
		return 0, nil
	}
	totalCardinalityLimitCheckDuration = time.Since(totalCardinalityLimitCheckStart)

	e.set.Logger.Debug("resourcesLimitCheckDuration",
		zap.Int64("duration", resourcesLimitCheckDuration.Milliseconds()),
		zap.String("datasource", ds.String()),
		zap.Int("records", len(records)),
	)
	e.set.Logger.Debug("totalCardinalityLimitCheckDuration",
		zap.Int64("duration", totalCardinalityLimitCheckDuration.Milliseconds()),
		zap.String("datasource", ds.String()),
		zap.Int("records", len(records)),
	)

	// Group by resourceFingerprint
	// resourceFp -> slice of attributeFp
	recordGroups := make(map[uint64][]uint64)
	indexByFp := make(map[uint64][]int) // resourceFp -> indices in 'records'
	resourceFps := make([]uint64, 0)
	exceedsCardinality := make(map[uint64]bool)
	for i, rec := range records {
		resourceFp := rec.resourceFingerprint
		if _, ok := recordGroups[resourceFp]; !ok {
			resourceFps = append(resourceFps, resourceFp)
		}
		recordGroups[resourceFp] = append(recordGroups[resourceFp], rec.fprint)
		indexByFp[resourceFp] = append(indexByFp[resourceFp], i)
	}

	e.set.Logger.Debug("resourceGroupsCount", zap.Int("count", len(recordGroups)), zap.String("datasource", ds.String()), zap.Int("records", len(records)))

	totalWrites := 0

	exceeds, err := e.keyCache.CardinalityLimitExceededMulti(ctx, resourceFps, ds)
	if err != nil {
		e.set.Logger.Debug("failed to check cardinality limit exceeded", zap.Error(err), zap.String("datasource", ds.String()), zap.Int("records", len(records)))
	}
	for i, exceeds := range exceeds {
		exceedsCardinality[resourceFps[i]] = exceeds
	}

	// For each resource, check which attrFps exist
	for resourceFp, attrFps := range recordGroups {
		// check cardinality limit
		if exceedsCardinality[resourceFp] {
			e.set.Logger.Info("cardinality limit exceeded", zap.Uint64("resourceFp", resourceFp), zap.String("ds", ds.String()))
			continue
		}

		existenceStart := time.Now()
		existence, err := e.keyCache.AttrsExistForResource(ctx, resourceFp, attrFps, ds)
		if err != nil {
			e.set.Logger.Debug("failed to check attrs existence", zap.Error(err), zap.String("datasource", ds.String()), zap.Int("records", len(records)))
			continue
		}
		existsCheckDuration += time.Since(existenceStart)

		// existence is parallel to attrFps
		indices := indexByFp[resourceFp]
		newAttrFps := make([]uint64, 0, len(attrFps))
		var newRecords []writeToStatementBatchRecord

		for j, exists := range existence {
			if !exists {
				idxInRecords := indices[j] // index in 'records'
				newAttrFps = append(newAttrFps, attrFps[j])
				newRecords = append(newRecords, records[idxInRecords])
			}
		}

		for _, nr := range newRecords {
			// TODO: handle error
			_ = stmt.Append(
				nr.roundedSixHrsUnixMilli,
				ds,
				nr.resourceFingerprint,
				nr.fprint,
				flattenJSONToStringMap(nr.rAttrs),
				flattenJSONToStringMap(nr.attrs),
			)
		}

		// We'll accumulate how many new records we wrote
		totalWrites += len(newRecords)

		// Add these new attrFps to the cache
		if len(newAttrFps) > 0 {
			addAttrsStart := time.Now()
			err := e.keyCache.AddAttrsToResource(ctx, resourceFp, newAttrFps, ds)
			if err != nil {
				e.set.Logger.Debug("failed to add to keyCache", zap.Error(err), zap.String("datasource", ds.String()), zap.Int("records", len(records)))
			}
			addAttrsDuration += time.Since(addAttrsStart)
		}
	}

	e.set.Logger.Debug("existsCheckDuration", zap.Int64("duration", existsCheckDuration.Milliseconds()), zap.String("datasource", ds.String()), zap.Int("records", len(records)))
	e.set.Logger.Debug("addAttrsDuration", zap.Int64("duration", addAttrsDuration.Milliseconds()), zap.String("datasource", ds.String()), zap.Int("records", len(records)))

	stmtStart := time.Now()
	if err := stmt.Send(); err != nil {
		return totalWrites, err
	}
	stmtDuration := time.Since(stmtStart)
	e.set.Logger.Debug("stmtDuration",
		zap.Int64("duration", stmtDuration.Milliseconds()),
		zap.String("datasource", ds.String()),
		zap.Int("records", len(records)),
	)

	return totalWrites, nil
}

// filterAttrs filters attributes based on the unique value tracker
func (e *metadataExporter) filterAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {

	var maxLen int
	switch datasource {
	case pipeline.SignalTraces.String():
		maxLen = int(e.cfg.MaxDistinctValues.Traces.MaxStringLength)
	case pipeline.SignalLogs.String():
		maxLen = int(e.cfg.MaxDistinctValues.Logs.MaxStringLength)
	case pipeline.SignalMetrics.String():
		maxLen = int(e.cfg.MaxDistinctValues.Metrics.MaxStringLength)
	}

	for k, v := range attrs {
		// if the attribute should be skipped, remove it
		if e.shouldSkipAttributeUVT(ctx, k, datasource) {
			delete(attrs, k)
			continue
		}
		switch v := v.(type) {
		case string:
			if len(v) == 0 || len(v) > maxLen {
				continue
			}
			e.addToUVT(ctx, makeUVTKey(k, datasource), v, datasource)
		default:
			// boolean, numbers would be skipped by the shouldSkipAttributeUVT
			continue
		}
	}
	return attrs
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	if !e.cfg.Enabled {
		return nil
	}
	stmt, err := e.conn.PrepareBatch(ctx, insertStmtQuery, driver.WithReleaseConnection())
	if err != nil {
		e.set.Logger.Error("failed to prepare batch", zap.Error(err), zap.String("pipeline", pipeline.SignalTraces.String()))
		return nil
	}
	defer stmt.Close()

	totalSpans := 0
	records := make([]writeToStatementBatchRecord, 0)

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

				span.Attributes().Range(func(attrKey string, v pcommon.Value) bool {
					if e.shouldSkipAttributeFromDB(ctx, attrKey, pipeline.SignalTraces.String()) {
						return true
					}
					spanAttrs[attrKey] = v.AsRaw()
					return true
				})
				spanAttrs["name"] = span.Name()

				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				filteredSpanAttrs := e.filterAttrs(ctx, flattenedSpanAttrs, pipeline.SignalTraces.String())
				spanFingerprint := fingerprint.FingerprintHash(filteredSpanAttrs)

				unixMilli := span.StartTimestamp().AsTime().UnixMilli()
				roundedSixHrsUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs

				records = append(records, writeToStatementBatchRecord{
					resourceFingerprint:    resourceFingerprint,
					fprint:                 spanFingerprint,
					rAttrs:                 flattenedResourceAttrs,
					attrs:                  filteredSpanAttrs,
					roundedSixHrsUnixMilli: roundedSixHrsUnixMilli,
				})
			}
		}
	}

	written, err := e.writeToStatementBatch(ctx, stmt, records, pipeline.SignalTraces)
	if err != nil {
		e.set.Logger.Error("failed to send stmt", zap.Error(err), zap.String("pipeline", pipeline.SignalTraces.String()))
	}
	skipped := totalSpans - written
	e.set.Logger.Debug("pushed traces attributes", zap.Int("total_spans", totalSpans), zap.Int("skipped_spans", skipped))
	return nil
}

func (e *metadataExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !e.cfg.Enabled {
		return nil
	}
	stmt, err := e.conn.PrepareBatch(ctx, insertStmtQuery, driver.WithReleaseConnection())
	if err != nil {
		e.set.Logger.Error("failed to prepare batch", zap.Error(err), zap.String("pipeline", pipeline.SignalMetrics.String()))
		return nil
	}
	defer stmt.Close()

	totalDps := 0
	records := make([]writeToStatementBatchRecord, 0)

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceAttrs := make(map[string]any, rm.Resource().Attributes().Len())
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		scopeMetrics := rm.ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				var pAttrs []pcommon.Map
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					totalDps += dps.Len()
					for l := 0; l < dps.Len(); l++ {
						pAttrs = append(pAttrs, dps.At(l).Attributes())
					}
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					totalDps += dps.Len()
					for l := 0; l < dps.Len(); l++ {
						pAttrs = append(pAttrs, dps.At(l).Attributes())
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					totalDps += dps.Len()
					for l := 0; l < dps.Len(); l++ {
						pAttrs = append(pAttrs, dps.At(l).Attributes())
					}
				case pmetric.MetricTypeExponentialHistogram:
					dps := metric.ExponentialHistogram().DataPoints()
					totalDps += dps.Len()
					for l := 0; l < dps.Len(); l++ {
						pAttrs = append(pAttrs, dps.At(l).Attributes())
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					totalDps += dps.Len()
					for l := 0; l < dps.Len(); l++ {
						pAttrs = append(pAttrs, dps.At(l).Attributes())
					}
				}

				for _, pAttr := range pAttrs {
					metricAttrs := make(map[string]any, pAttr.Len())
					pAttr.Range(func(k string, v pcommon.Value) bool {
						metricAttrs[k] = v.AsRaw()
						return true
					})

					flattenedMetricAttrs := flatten.FlattenJSON(metricAttrs, "")
					metricFingerprint := fingerprint.FingerprintHash(flattenedMetricAttrs)
					unixMilli := time.Now().UnixMilli()
					roundedSixHrsUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs

					records = append(records, writeToStatementBatchRecord{
						resourceFingerprint:    resourceFingerprint,
						fprint:                 metricFingerprint,
						rAttrs:                 flattenedResourceAttrs,
						attrs:                  flattenedMetricAttrs,
						roundedSixHrsUnixMilli: roundedSixHrsUnixMilli,
					})

				}
			}
		}
	}

	written, err := e.writeToStatementBatch(ctx, stmt, records, pipeline.SignalMetrics)
	if err != nil {
		e.set.Logger.Error("failed to send stmt", zap.Error(err), zap.String("pipeline", pipeline.SignalMetrics.String()))
	}
	skipped := totalDps - written
	e.set.Logger.Debug("pushed metrics attributes", zap.Int("total_dps", totalDps), zap.Int("skipped_dps", skipped))
	return nil
}

func (e *metadataExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	if !e.cfg.Enabled {
		return nil
	}
	stmt, err := e.conn.PrepareBatch(ctx, insertStmtQuery, driver.WithReleaseConnection())
	if err != nil {
		e.set.Logger.Error("failed to prepare batch", zap.Error(err), zap.String("pipeline", pipeline.SignalLogs.String()))
		return nil
	}
	defer stmt.Close()

	totalLogRecords := 0
	records := make([]writeToStatementBatchRecord, 0)

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resourceAttrs := make(map[string]any, rl.Resource().Attributes().Len())
		rl.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if e.shouldSkipAttributeFromDB(ctx, k, pipeline.SignalLogs.String()) {
				return true
			}
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				totalLogRecords++
				logRecord := logs.At(k)
				logRecordAttrs := make(map[string]any, logRecord.Attributes().Len())

				logRecord.Attributes().Range(func(attrKey string, v pcommon.Value) bool {
					if e.shouldSkipAttributeFromDB(ctx, attrKey, pipeline.SignalLogs.String()) {
						return true
					}
					logRecordAttrs[attrKey] = v.AsRaw()
					return true
				})

				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				filteredLogRecordAttrs := e.filterAttrs(ctx, flattenedLogRecordAttrs, pipeline.SignalLogs.String())
				logRecordFingerprint := fingerprint.FingerprintHash(filteredLogRecordAttrs)

				unixMilli := logRecord.Timestamp().AsTime().UnixMilli()
				roundedSixHrsUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs

				records = append(records, writeToStatementBatchRecord{
					resourceFingerprint:    resourceFingerprint,
					fprint:                 logRecordFingerprint,
					rAttrs:                 flattenedResourceAttrs,
					attrs:                  filteredLogRecordAttrs,
					roundedSixHrsUnixMilli: roundedSixHrsUnixMilli,
				})
			}
		}
	}

	written, err := e.writeToStatementBatch(ctx, stmt, records, pipeline.SignalLogs)
	if err != nil {
		e.set.Logger.Error("failed to send stmt", zap.Error(err), zap.String("pipeline", pipeline.SignalLogs.String()))
	}
	skipped := totalLogRecords - written
	e.set.Logger.Debug("pushed logs attributes", zap.Int("total_log_records", totalLogRecords), zap.Int("skipped_log_records", skipped))
	return nil
}
