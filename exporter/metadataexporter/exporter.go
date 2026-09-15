package metadataexporter

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/internal/common/spanfields"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	kash "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/cache"
)

const (
	valuTrackerKeysTTL = 45 * time.Minute // ttl for keys in value tracker
	// skipDecisionTTL is how long a key stays skipped after the periodic
	// tag_attributes_v2 count last found it over the limit.
	skipDecisionTTL = 24 * time.Hour
	// metadataRetention matches the TTL of attributes_metadata.
	metadataRetention = 30 * 24 * time.Hour
	// maxFutureSkew is how far ahead of the collector clock a record timestamp
	// may be before the current time is used instead.
	maxFutureSkew = time.Hour
	// maxExecutionTimeSetting is the ClickHouse setting that bounds an INSERT
	// server-side; it is set from the exporter timeout unless the DSN sets it.
	maxExecutionTimeSetting = "max_execution_time"
	insertStmtQuery         = "INSERT INTO signoz_metadata.distributed_attributes_metadata (unix_milli, data_source, resource_fingerprint, attrs_fingerprint, resource_attributes, attributes, intrinsic_attributes)"
	// intrinsicTrackerPrefix keeps intrinsic field names apart from attribute
	// keys of the same name in the value tracker.
	intrinsicTrackerPrefix = "intrinsic:"
	meterName              = "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter"
)

// drop reasons recorded on the values_dropped counter.
const (
	dropReasonEmpty             = "empty"
	dropReasonOversized         = "oversized"
	dropReasonResourceOversized = "resource_oversized"
	dropReasonCardinality       = "cardinality"
)

type tagValueCountFromDB struct {
	tagDataType         string
	stringTagValueCount uint64
	numberValueCount    uint64
	// skippedUntil is set when the count was found over the limit; the key
	// stays skipped until then even if a later count falls under the limit.
	skippedUntil time.Time
}

// exceeds reports whether the DB-derived count puts the key over the limit.
func (c tagValueCountFromDB) exceeds(limits LimitsConfig) bool {
	switch c.tagDataType {
	case "string":
		return c.stringTagValueCount > limits.MaxStringDistinctValues
	case "float64", "int64":
		return true
	}
	return false
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

	// logsMetadataWriters is the ordered list of writers dispatched in parallel on
	// every PushLogs call.
	logsMetadataWriters []LogsMetadataWriter

	valuesDropped metric.Int64Counter
	rowsWritten   metric.Int64Counter
	insertErrors  metric.Int64Counter
}

type writeToStatementBatchRecord struct {
	resourceFingerprint    uint64
	fprint                 uint64
	rAttrs                 map[string]any
	attrs                  map[string]any
	intrinsics             map[string]string
	roundedSixHrsUnixMilli int64
}

// setFingerprint identifies an attribute set by its attributes and its
// intrinsic fields together. The two hashes are combined in order, so equal
// maps do not cancel out and swapped maps give a different set.
func setFingerprint(attrs map[string]any, intrinsics map[string]string) uint64 {
	fp := fingerprint.FingerprintHash(attrs)
	if len(intrinsics) == 0 {
		return fp
	}
	return fp ^ (fingerprint.FingerprintHashStrings(intrinsics) + 0x9e3779b97f4a7c15 + (fp << 6) + (fp >> 2))
}

// intrinsicFieldNames are the keys written to the intrinsic map; their
// tracker keys are built once.
var intrinsicFieldNames = []string{
	"name", "kind_string", "status_code_string", "has_error", "is_remote",
	"http_method", "http_host", "http_url", "response_status_code", "db_name", "db_operation",
	"external_http_method", "external_http_url",
	"severity_text", "severity_number",
}

var intrinsicTrackerKeys = func() map[string]string {
	keys := make(map[string]string, len(intrinsicFieldNames))
	for _, name := range intrinsicFieldNames {
		keys[name] = intrinsicTrackerPrefix + name
	}
	return keys
}()

func intrinsicTrackerKey(name string) string {
	if key, ok := intrinsicTrackerKeys[name]; ok {
		return key
	}
	return intrinsicTrackerPrefix + name
}

// spanIntrinsics returns the span's intrinsic and calculated fields under the
// names the fields API uses for the span context. Empty calculated fields are
// left out.
func spanIntrinsics(span ptrace.Span) map[string]string {
	c := spanfields.CalculatedFrom(span.Attributes(), span.Kind())
	hasError := "false"
	if span.Status().Code() == ptrace.StatusCodeError {
		hasError = "true"
	}
	m := make(map[string]string, 13)
	m["name"] = span.Name()
	m["kind_string"] = span.Kind().String()
	m["status_code_string"] = span.Status().Code().String()
	m["has_error"] = hasError
	m["is_remote"] = spanfields.IsRemote(span.Flags())
	putNonEmpty(m, "http_method", c.HttpMethod)
	putNonEmpty(m, "http_host", c.HttpHost)
	putNonEmpty(m, "http_url", c.HttpUrl)
	putNonEmpty(m, "response_status_code", c.ResponseStatusCode)
	putNonEmpty(m, "db_name", c.DBName)
	putNonEmpty(m, "db_operation", c.DBOperation)
	putNonEmpty(m, "external_http_method", c.ExternalHttpMethod)
	putNonEmpty(m, "external_http_url", c.ExternalHttpUrl)
	return m
}

func putNonEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// logIntrinsics returns the log record's severity fields under the names the
// fields API uses for the log context.
func logIntrinsics(lr plog.LogRecord) map[string]string {
	m := make(map[string]string, 2)
	if lr.SeverityText() != "" {
		m["severity_text"] = lr.SeverityText()
	}
	if lr.SeverityNumber() != plog.SeverityNumberUnspecified {
		m["severity_number"] = strconv.Itoa(int(lr.SeverityNumber()))
	}
	return m
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

func newMetadataExporter(ctx context.Context, cfg Config, set exporter.Settings) (*metadataExporter, error) {
	opts, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if opts.Settings == nil {
		opts.Settings = clickhouse.Settings{}
	}
	if _, ok := opts.Settings[maxExecutionTimeSetting]; !ok && cfg.Timeout > 0 {
		opts.Settings[maxExecutionTimeSetting] = int(cfg.Timeout.Seconds())
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}
	return newMetadataExporterWithConn(ctx, cfg, set, conn)
}

func newMetadataExporterWithConn(ctx context.Context, cfg Config, set exporter.Settings, conn driver.Conn) (*metadataExporter, error) {
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

			TracesTTL:  cfg.MaxDistinctValues.Traces.Bucket,
			MetricsTTL: cfg.MaxDistinctValues.Metrics.Bucket,
			LogsTTL:    cfg.MaxDistinctValues.Logs.Bucket,

			TracesWindow:  cfg.MaxDistinctValues.Traces.Bucket,
			MetricsWindow: cfg.MaxDistinctValues.Metrics.Bucket,
			LogsWindow:    cfg.MaxDistinctValues.Logs.Bucket,

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
			TracesFingerprintCacheTTL:        cfg.MaxDistinctValues.Traces.Bucket,
			MetricsFingerprintCacheTTL:       cfg.MaxDistinctValues.Metrics.Bucket,
			LogsFingerprintCacheTTL:          cfg.MaxDistinctValues.Logs.Bucket,
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

	meter := set.MeterProvider.Meter(meterName)
	valuesDropped, err := meter.Int64Counter(
		"signoz_metadata_exporter_values_dropped",
		metric.WithDescription("Attribute values left out of the metadata table, by signal and reason"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exporter metrics")
	}
	rowsWritten, err := meter.Int64Counter(
		"signoz_metadata_exporter_rows_written",
		metric.WithDescription("Rows appended to the metadata table insert, by signal"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exporter metrics")
	}
	insertErrors, err := meter.Int64Counter(
		"signoz_metadata_exporter_insert_errors",
		metric.WithDescription("Failed inserts into the metadata table, by signal"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exporter metrics")
	}

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

		valuesDropped: valuesDropped,
		rowsWritten:   rowsWritten,
		insertErrors:  insertErrors,
	}

	e.logTagValueCountFromDB.Store(initMap())
	e.tracesTagValueCountFromDB.Store(initMap())
	e.metricsTagValueCountFromDB.Store(initMap())

	e.logsMetadataWriters = []LogsMetadataWriter{
		newAttributeMetadataWriter(e),
	}
	if cfg.JSON.Enabled {
		jsonProc, err := newJSONMetadataWriter(
			ctx,
			cfg.JSON,
			set.Logger,
			e,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create json processor")
		}
		e.logsMetadataWriters = append(e.logsMetadataWriters, jsonProc)
	}

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
	defer func() { _ = rows.Close() }()

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
	e.storeTagValuesAtomic(&e.logTagValueCountFromDB, newValues, e.cfg.MaxDistinctValues.Logs, time.Now())
}

func (e *metadataExporter) storeTracesTagValues(newValues map[string]tagValueCountFromDB) {
	e.storeTagValuesAtomic(&e.tracesTagValueCountFromDB, newValues, e.cfg.MaxDistinctValues.Traces, time.Now())
}

// storeTagValuesAtomic replaces the DB-derived counts. A key found over the
// limit is marked skipped for skipDecisionTTL, and a previously marked key is
// carried over while its mark lasts if the new counts no longer put it over
// the limit; the other exporters stop writing such keys to tag_attributes_v2,
// which would otherwise re-admit them here on the next refresh.
func (e *metadataExporter) storeTagValuesAtomic(target *atomic.Pointer[map[string]tagValueCountFromDB], newValues map[string]tagValueCountFromDB, limits LimitsConfig, now time.Time) {
	for key, val := range newValues {
		if val.exceeds(limits) {
			val.skippedUntil = now.Add(skipDecisionTTL)
			newValues[key] = val
		}
	}
	if prev := target.Load(); prev != nil {
		for key, val := range *prev {
			if !val.skippedUntil.After(now) {
				continue
			}
			if cur, ok := newValues[key]; ok && cur.exceeds(limits) {
				continue
			}
			newValues[key] = val
		}
	}
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
	return val.exceeds(cfgMax)
}

// signalGuards returns the value tracker, the always-include set and the
// limits for the signal. ok is false for an unknown signal.
func (e *metadataExporter) signalGuards(datasource string) (tracker *ValueTracker, alwaysInclude map[string]struct{}, limits LimitsConfig, ok bool) {
	switch datasource {
	case pipeline.SignalTraces.String():
		return e.tracesTracker, e.alwaysIncludeTracesAttributes, e.cfg.MaxDistinctValues.Traces, true
	case pipeline.SignalMetrics.String():
		return e.metricsTracker, e.alwaysIncludeMetricsAttributes, e.cfg.MaxDistinctValues.Metrics, true
	case pipeline.SignalLogs.String():
		return e.logsTracker, e.alwaysIncludeLogsAttributes, e.cfg.MaxDistinctValues.Logs, true
	}
	return nil, nil, LimitsConfig{}, false
}

// shouldSkipAttributeUVT checks if an attribute should be skipped based on the unique value tracker
func (e *metadataExporter) shouldSkipAttributeUVT(_ context.Context, key, datasource string) bool {
	tracker, alwaysInclude, _, ok := e.signalGuards(datasource)
	if !ok {
		return false
	}
	if _, ok := alwaysInclude[key]; ok {
		return false
	}
	if e.getType(key, datasource) == utils.FieldDataTypeFloat64.String() {
		return true
	}
	return tracker.IsOverLimit(key)
}

func (e *metadataExporter) recordDrop(ctx context.Context, datasource, reason string) {
	e.valuesDropped.Add(ctx, 1, metric.WithAttributes(
		attribute.String("signal", datasource),
		attribute.String("reason", reason),
	))
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
			e.set.Logger.Debug("cardinality limit exceeded", zap.Uint64("resourceFp", resourceFp), zap.String("ds", ds.String()))
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
			if err := stmt.Append(
				nr.roundedSixHrsUnixMilli,
				ds,
				nr.resourceFingerprint,
				nr.fprint,
				flattenJSONToStringMap(nr.rAttrs),
				flattenJSONToStringMap(nr.attrs),
				nr.intrinsics,
			); err != nil {
				e.set.Logger.Debug("failed to append record", zap.Error(err), zap.String("datasource", ds.String()))
			}
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
		e.insertErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("signal", ds.String())))
		return totalWrites, err
	}
	e.rowsWritten.Add(ctx, int64(totalWrites), metric.WithAttributes(attribute.String("signal", ds.String())))
	stmtDuration := time.Since(stmtStart)
	e.set.Logger.Debug("stmtDuration",
		zap.Int64("duration", stmtDuration.Milliseconds()),
		zap.String("datasource", ds.String()),
		zap.Int("records", len(records)),
	)

	return totalWrites, nil
}

// filterAttrs removes the attributes that must not reach the metadata table:
// keys the value tracker has flagged, empty strings, strings over
// max_string_length (the key is then flagged so its shorter values are dropped
// too), keys whose DB-derived type is float64, and string or numeric values
// that take a key past max_string_distinct_values. Bools are passed through.
func (e *metadataExporter) filterAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	return e.filterValues(ctx, attrs, datasource)
}

// filterIntrinsics applies the attribute rules to the intrinsic field map. An
// oversized value is dropped from its row only: one long span name must not
// take the name field out of every other span.
func (e *metadataExporter) filterIntrinsics(ctx context.Context, fields map[string]string, datasource string) map[string]string {
	tracker, alwaysInclude, limits, ok := e.signalGuards(datasource)
	if !ok {
		return fields
	}
	maxLen := int(limits.MaxStringLength)

	for k, v := range fields {
		if _, ok := alwaysInclude[k]; ok {
			continue
		}
		trackerKey := intrinsicTrackerKey(k)
		if tracker.IsOverLimit(trackerKey) {
			delete(fields, k)
			e.recordDrop(ctx, datasource, dropReasonCardinality)
			continue
		}
		if len(v) == 0 {
			delete(fields, k)
			continue
		}
		if len(v) > maxLen {
			delete(fields, k)
			e.recordDrop(ctx, datasource, dropReasonOversized)
			continue
		}
		if tracker.AddString(trackerKey, v) {
			delete(fields, k)
			e.recordDrop(ctx, datasource, dropReasonCardinality)
		}
	}
	return fields
}

func (e *metadataExporter) filterValues(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	tracker, alwaysInclude, limits, ok := e.signalGuards(datasource)
	if !ok {
		return attrs
	}
	maxLen := int(limits.MaxStringLength)

	for k, v := range attrs {
		if _, ok := alwaysInclude[k]; ok {
			continue
		}
		if e.getType(k, datasource) == utils.FieldDataTypeFloat64.String() || tracker.IsOverLimit(k) {
			delete(attrs, k)
			e.recordDrop(ctx, datasource, dropReasonCardinality)
			continue
		}
		switch v := v.(type) {
		case string:
			if len(v) == 0 {
				delete(attrs, k)
				e.recordDrop(ctx, datasource, dropReasonEmpty)
				continue
			}
			if len(v) > maxLen {
				tracker.MarkOverLimit(k)
				delete(attrs, k)
				e.recordDrop(ctx, datasource, dropReasonOversized)
				continue
			}
			if tracker.AddString(k, v) {
				delete(attrs, k)
				e.recordDrop(ctx, datasource, dropReasonCardinality)
			}
		case int64, float64:
			if tracker.AddValue(k, v) {
				delete(attrs, k)
				e.recordDrop(ctx, datasource, dropReasonCardinality)
			}
		}
	}
	return attrs
}

// filterResourceAttrs removes resource attribute values longer than
// max_resource_string_length unless the key is in always_include_attributes.
func (e *metadataExporter) filterResourceAttrs(ctx context.Context, attrs map[string]any, datasource string) map[string]any {
	_, alwaysInclude, limits, ok := e.signalGuards(datasource)
	if !ok {
		return attrs
	}
	maxLen := int(limits.MaxResourceStringLength)

	for k, v := range attrs {
		s, isString := v.(string)
		if !isString || len(s) <= maxLen {
			continue
		}
		if _, ok := alwaysInclude[k]; ok {
			continue
		}
		delete(attrs, k)
		e.recordDrop(ctx, datasource, dropReasonResourceOversized)
	}
	return attrs
}

// bucketStart returns the start of the write bucket that holds ts. A zero
// timestamp, one older than the metadata retention or one more than
// maxFutureSkew ahead of now is replaced by now.
func bucketStart(ts, now time.Time, bucket time.Duration) int64 {
	if ts.IsZero() || ts.Unix() == 0 || ts.Before(now.Add(-metadataRetention)) || ts.After(now.Add(maxFutureSkew)) {
		ts = now
	}
	bucketMs := bucket.Milliseconds()
	return (ts.UnixMilli() / bucketMs) * bucketMs
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
	defer func() { _ = stmt.Close() }()

	totalSpans := 0
	records := make([]writeToStatementBatchRecord, 0)
	now := time.Now()

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
		flattenedResourceAttrs := e.filterResourceAttrs(ctx, flatten.FlattenJSON(resourceAttrs, ""), pipeline.SignalTraces.String())
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
				intrinsics := e.filterIntrinsics(ctx, spanIntrinsics(span), pipeline.SignalTraces.String())

				flattenedSpanAttrs := flatten.FlattenJSON(spanAttrs, "")
				filteredSpanAttrs := e.filterAttrs(ctx, flattenedSpanAttrs, pipeline.SignalTraces.String())
				// The span name stays in attributes until the query side reads
				// intrinsic_attributes.
				if name, ok := intrinsics["name"]; ok {
					filteredSpanAttrs["name"] = name
				}
				spanFingerprint := setFingerprint(filteredSpanAttrs, intrinsics)

				roundedSixHrsUnixMilli := bucketStart(span.StartTimestamp().AsTime(), now, e.cfg.MaxDistinctValues.Traces.Bucket)

				records = append(records, writeToStatementBatchRecord{
					resourceFingerprint:    resourceFingerprint,
					fprint:                 spanFingerprint,
					rAttrs:                 flattenedResourceAttrs,
					attrs:                  filteredSpanAttrs,
					intrinsics:             intrinsics,
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
	defer func() { _ = stmt.Close() }()

	totalDps := 0
	records := make([]writeToStatementBatchRecord, 0)
	now := time.Now()

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceAttrs := make(map[string]any, rm.Resource().Attributes().Len())
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := e.filterResourceAttrs(ctx, flatten.FlattenJSON(resourceAttrs, ""), pipeline.SignalMetrics.String())
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

					flattenedMetricAttrs := e.filterAttrs(ctx, flatten.FlattenJSON(metricAttrs, ""), pipeline.SignalMetrics.String())
					metricFingerprint := fingerprint.FingerprintHash(flattenedMetricAttrs)
					roundedSixHrsUnixMilli := bucketStart(now, now, e.cfg.MaxDistinctValues.Metrics.Bucket)

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
	g, gCtx := errgroup.WithContext(ctx)
	for _, proc := range e.logsMetadataWriters {
		g.Go(func() error { return proc.Process(gCtx, ld) })
	}
	if err := g.Wait(); err != nil {
		e.set.Logger.Error("logs processor error", zap.Error(err))
	}
	return nil
}
