package signozclickhouseauditexporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/goccy/go-json"
	"github.com/jellydator/ttlcache/v3"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

type logsExporter struct {
	logger            *zap.Logger
	cfg               *Config
	db                clickhouse.Conn
	insertLogsSQL     string
	insertResourceSQL string
	meterProvider     metric.MeterProvider
	durationHistogram metric.Float64Histogram
	keysCache         *ttlcache.Cache[string, struct{}]
	rfCache           *ttlcache.Cache[string, struct{}]
}

func newExporter(logger *zap.Logger, cfg *Config, meterProvider metric.MeterProvider) *logsExporter {
	return &logsExporter{
		logger:            logger,
		cfg:               cfg,
		insertLogsSQL:     fmt.Sprintf(insertLogsSQLTemplate, databaseName, distributedLogsTable),
		insertResourceSQL: fmt.Sprintf(insertLogsResourceSQLTemplate, databaseName, distributedLogsResource),
		meterProvider:     meterProvider,
	}
}

func (e *logsExporter) start(_ context.Context, _ component.Host) error {
	options, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return fmt.Errorf("failed to build clickhouse options: %w", err)
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return fmt.Errorf("failed to ping clickhouse: %w", err)
	}
	e.db = db

	e.keysCache = ttlcache.New(
		ttlcache.WithTTL[string, struct{}](240*time.Minute),
		ttlcache.WithCapacity[string, struct{}](50000),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
	)
	go e.keysCache.Start()

	e.rfCache = ttlcache.New(
		ttlcache.WithTTL[string, struct{}](distributedLogsResourceSeconds*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go e.rfCache.Start()

	meter := e.meterProvider.Meter(metadata.ScopeName)
	e.durationHistogram, err = meter.Float64Histogram(
		"exporter_db_write_latency",
		metric.WithDescription("Time taken to write audit data to ClickHouse"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
		return fmt.Errorf("failed to create duration histogram: %w", err)
	}

	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.keysCache != nil {
		e.keysCache.Stop()
	}

	if e.rfCache != nil {
		e.rfCache.Stop()
	}

	if e.db != nil {
		return e.db.Close()
	}

	return nil
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()

	tagStmt, err := e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedTagAttributes), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("PrepareTagBatch: %w", err)
	}
	defer func() {
		if err := tagStmt.Close(); err != nil {
			e.logger.Warn("failed to close tag batch", zap.Error(err))
		}
	}()

	attrKeysStmt, err := e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedLogsAttributeKeys), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("PrepareAttributeKeysBatch: %w", err)
	}
	defer func() {
		if err := attrKeysStmt.Close(); err != nil {
			e.logger.Warn("failed to close attribute keys batch", zap.Error(err))
		}
	}()

	resourceKeysStmt, err := e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedLogsResourceKeys), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("PrepareResourceKeysBatch: %w", err)
	}
	defer func() {
		if err := resourceKeysStmt.Close(); err != nil {
			e.logger.Warn("failed to close resource keys batch", zap.Error(err))
		}
	}()

	insertLogsStmt, err := e.db.PrepareBatch(ctx, e.insertLogsSQL, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("PrepareLogsBatch: %w", err)
	}
	defer func() {
		if err := insertLogsStmt.Close(); err != nil {
			e.logger.Warn("failed to close logs batch", zap.Error(err))
		}
	}()

	// Track resource fingerprints: bucket -> resourceJSON -> fingerprint
	resourcesSeen := make(map[int64]map[string]string)

	// Process records
	processStart := time.Now()

	for i := range ld.ResourceLogs().Len() {
		logs := ld.ResourceLogs().At(i)
		res := logs.Resource()
		resourcesMap := convertAttributes(res.Attributes(), true)
		serializedRes, err := json.Marshal(resourcesMap.StringData)
		if err != nil {
			return fmt.Errorf("failed to marshal resource attributes: %w", err)
		}
		resourceJSON := string(serializedRes)

		for j := range logs.ScopeLogs().Len() {
			scope := logs.ScopeLogs().At(j).Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeMap := convertAttributes(scope.Attributes(), true)

			records := logs.ScopeLogs().At(j).LogRecords()
			for k := range records.Len() {
				record := records.At(k)

				ts := uint64(record.Timestamp())
				ots := uint64(record.ObservedTimestamp())
				if ots == 0 {
					ots = uint64(time.Now().UnixNano())
				}
				if ts == 0 {
					ts = ots
				}

				id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
				if err != nil {
					return fmt.Errorf("failed to generate id: %w", err)
				}

				lBucketStart := bucketTimestamp(int64(ts/1000000000), distributedLogsResourceSeconds)

				fp := resolveFingerprint(resourcesSeen, lBucketStart, resourceJSON, res)
				attrsMap := convertAttributes(record.Attributes(), false)

				e.appendTagAttributes(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeResource, resourcesMap)
				e.appendTagAttributes(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeScope, scopeMap)
				e.appendTagAttributes(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeAttribute, attrsMap)

				columnValues := make([]any, 0, 19)
				columnValues = append(columnValues,
					uint64(lBucketStart),
					fp,
					ts,
					ots,
					id.String(),
					utils.TraceIDToHexOrEmptyString(record.TraceID()),
					utils.SpanIDToHexOrEmptyString(record.SpanID()),
					uint32(record.Flags()),
					record.SeverityText(),
					uint8(record.SeverityNumber()),
					record.Body().AsString(),
					scopeName,
					scopeVersion,
					scopeMap.StringData,
					attrsMap.StringData,
					attrsMap.NumberData,
					attrsMap.BoolData,
					resourcesMap.StringData,
					record.EventName(),
				)

				if err := insertLogsStmt.Append(columnValues...); err != nil {
					return fmt.Errorf("failed to append log row: %w", err)
				}
			}
		}
	}

	processDuration := time.Since(processStart)

	// Prepare and append resource fingerprints
	insertResourcesStmt, err := e.db.PrepareBatch(ctx, e.insertResourceSQL, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare resources statement: %w", err)
	}
	defer func() {
		if err := insertResourcesStmt.Close(); err != nil {
			e.logger.Warn("failed to close resources batch", zap.Error(err))
		}
	}()

	for bucketTs, bucket := range resourcesSeen {
		for resourceLabels, fp := range bucket {
			key := utils.MakeKeyForRFCache(bucketTs, fp)
			if e.rfCache.Get(key) != nil {
				continue
			}
			if err := insertResourcesStmt.Append(resourceLabels, fp, bucketTs); err != nil {
				return fmt.Errorf("failed to append resource row: %w", err)
			}
			e.rfCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
		}
	}

	// Flush all batches in parallel
	networkStart := time.Now()

	errc := make(chan error, 5)
	go func() { errc <- flushBatch(ctx, insertLogsStmt, distributedLogsTable, e.durationHistogram) }()
	go func() { errc <- flushBatch(ctx, insertResourcesStmt, distributedLogsResource, e.durationHistogram) }()
	go func() { errc <- flushBatch(ctx, attrKeysStmt, distributedLogsAttributeKeys, e.durationHistogram) }()
	go func() { errc <- flushBatch(ctx, resourceKeysStmt, distributedLogsResourceKeys, e.durationHistogram) }()
	go func() { errc <- flushBatch(ctx, tagStmt, distributedTagAttributes, e.durationHistogram) }()

	var errs []error
	for range 5 {
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}

	networkDuration := time.Since(networkStart)

	if len(errs) > 0 {
		return fmt.Errorf("failed to send batches: %w", errors.Join(errs...))
	}

	e.logger.Debug("insert audit logs",
		zap.Int("records", ld.LogRecordCount()),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", time.Since(start).String()),
	)

	return nil
}

func (e *logsExporter) appendTagAttributes(tagStmt driver.Batch, attrKeysStmt driver.Batch, resourceKeysStmt driver.Batch, tagType utils.TagType, attrs typedAttributes) {
	unixMilli := (time.Now().UnixMilli() / 3600000) * 3600000
	now := time.Now()

	for key, val := range attrs.StringData {
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeString, now)
		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeString, val, (*int64)(nil), (*float64)(nil))
	}

	for key, val := range attrs.NumberData {
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeNumber, now)
		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeNumber, "", (*int64)(nil), &val)
	}

	for key := range attrs.BoolData {
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeBool, now)
		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeBool, "", (*int64)(nil), (*float64)(nil))
	}
}

func (e *logsExporter) appendAttributeKey(attrKeysStmt driver.Batch, resourceKeysStmt driver.Batch, key string, tagType utils.TagType, datatype utils.TagDataType, now time.Time) {
	cacheKey := utils.MakeKeyForAttributeKeys(key, tagType, datatype)
	if item := e.keysCache.Get(cacheKey); item != nil {
		return
	}

	switch tagType {
	case utils.TagTypeResource:
		_ = resourceKeysStmt.Append(key, string(datatype), now)
	case utils.TagTypeAttribute:
		_ = attrKeysStmt.Append(key, string(datatype), now)
	}

	e.keysCache.Set(cacheKey, struct{}{}, ttlcache.DefaultTTL)
}
