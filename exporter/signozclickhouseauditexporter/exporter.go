package signozclickhouseauditexporter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/goccy/go-json"
	"github.com/jellydator/ttlcache/v3"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
)

const (
	distributedLogsTable           = "distributed_logs"
	distributedLogsResource        = "distributed_logs_resource"
	distributedLogsAttributeKeys   = "distributed_logs_attribute_keys"
	distributedLogsResourceKeys    = "distributed_logs_resource_keys"
	distributedTagAttributes       = "distributed_tag_attributes"
	distributedLogsResourceSeconds = 1800

	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s.%s (
		ts_bucket_start,
		resource_fingerprint,
		timestamp,
		observed_timestamp,
		id,
		trace_id,
		span_id,
		trace_flags,
		severity_text,
		severity_number,
		body,
		scope_name,
		scope_version,
		scope_string,
		attributes_string,
		attributes_number,
		attributes_bool,
		resource,
		event_name
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?
	)`

	// language=ClickHouse SQL
	insertLogsResourceSQLTemplate = `INSERT INTO %s.%s (
		labels,
		fingerprint,
		seen_at_ts_bucket_start
	) VALUES (?, ?, ?)`
)

type attributeMap struct {
	StringData map[string]string
	NumberData map[string]float64
	BoolData   map[string]bool
}

type statementSendDuration struct {
	Name     string
	duration time.Duration
}

type signozAuditExporter struct {
	db                clickhouse.Conn
	insertLogsSQL     string
	insertResourceSQL string

	logger *zap.Logger
	cfg    *Config

	wg        *sync.WaitGroup
	closeChan chan struct{}

	durationHistogram metric.Float64Histogram
	keysCache         *ttlcache.Cache[string, struct{}]
	rfCache           *ttlcache.Cache[string, struct{}]
}

func newExporter(
	logger *zap.Logger,
	cfg *Config,
	keysCache *ttlcache.Cache[string, struct{}],
	rfCache *ttlcache.Cache[string, struct{}],
	meter metric.Meter,
) (*signozAuditExporter, error) {
	durationHistogram, err := meter.Float64Histogram(
		"exporter_db_write_latency",
		metric.WithDescription("Time taken to write audit data to ClickHouse"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating duration histogram: %w", err)
	}

	return &signozAuditExporter{
		insertLogsSQL:     fmt.Sprintf(insertLogsSQLTemplate, databaseName, distributedLogsTable),
		insertResourceSQL: fmt.Sprintf(insertLogsResourceSQLTemplate, databaseName, distributedLogsResource),
		logger:            logger,
		cfg:               cfg,
		wg:                new(sync.WaitGroup),
		closeChan:         make(chan struct{}),
		durationHistogram: durationHistogram,
		keysCache:         keysCache,
		rfCache:           rfCache,
	}, nil
}

// start opens the ClickHouse connection. Called by the collector lifecycle,
// not during factory construction — so config validation doesn't make network calls.
func (e *signozAuditExporter) start(_ context.Context, _ component.Host) error {
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
	return nil
}

func (e *signozAuditExporter) shutdown(_ context.Context) error {
	close(e.closeChan)
	e.wg.Wait()

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

func (e *signozAuditExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	err := e.pushToClickhouse(ctx, ld)
	if err != nil {
		if strings.Contains(err.Error(), "code: 252") {
			e.logger.Warn("too many partitions for single INSERT block, dropping the batch")
			return nil
		}
		return err
	}
	return nil
}

func (e *signozAuditExporter) pushToClickhouse(ctx context.Context, ld plog.Logs) error {
	start := time.Now()
	const batchCount = 5

	select {
	case <-e.closeChan:
		return fmt.Errorf("shutdown has been called")
	default:
	}

	// Prepare batch statements
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
		resourcesMap := attributesToMap(res.Attributes(), true)
		serializedRes, err := json.Marshal(resourcesMap.StringData)
		if err != nil {
			return fmt.Errorf("failed to marshal resource attributes: %w", err)
		}
		resourceJSON := string(serializedRes)

		for j := range logs.ScopeLogs().Len() {
			scope := logs.ScopeLogs().At(j).Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeMap := attributesToMap(scope.Attributes(), true)

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

				lBucketStart := tsBucket(int64(ts/1000000000), distributedLogsResourceSeconds)

				fp := getOrCreateFingerprint(resourcesSeen, lBucketStart, resourceJSON, res)
				attrsMap := attributesToMap(record.Attributes(), false)

				// Add attributes to tag and keys statements
				e.addAttrsToTagStatement(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeResource, resourcesMap)
				e.addAttrsToTagStatement(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeScope, scopeMap)
				e.addAttrsToTagStatement(tagStmt, attrKeysStmt, resourceKeysStmt, utils.TagTypeAttribute, attrsMap)

				// Build column values as slice (19 columns)
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
		return fmt.Errorf("PrepareResourcesBatch: %w", err)
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

	var wg sync.WaitGroup
	chErr := make(chan error, batchCount)
	chDuration := make(chan statementSendDuration, batchCount)

	wg.Add(batchCount)
	go send(insertLogsStmt, distributedLogsTable, chDuration, chErr, &wg)
	go send(insertResourcesStmt, distributedLogsResource, chDuration, chErr, &wg)
	go send(attrKeysStmt, distributedLogsAttributeKeys, chDuration, chErr, &wg)
	go send(resourceKeysStmt, distributedLogsResourceKeys, chDuration, chErr, &wg)
	go send(tagStmt, distributedTagAttributes, chDuration, chErr, &wg)
	wg.Wait()
	close(chErr)

	networkDuration := time.Since(networkStart)

	for range batchCount {
		d := <-chDuration
		e.durationHistogram.Record(ctx, float64(d.duration.Milliseconds()),
			metric.WithAttributes(
				attribute.String("table", d.Name),
				attribute.String("exporter", pipeline.SignalLogs.String()),
			),
		)
	}

	for err := range chErr {
		if err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	e.logger.Debug("insert audit logs",
		zap.Int("records", ld.LogRecordCount()),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", time.Since(start).String()),
	)

	return nil
}

func (e *signozAuditExporter) addAttrsToTagStatement(
	tagStmt driver.Batch,
	attrKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	tagType utils.TagType,
	attrs attributeMap,
) {
	unixMilli := (time.Now().UnixMilli() / 3600000) * 3600000
	now := time.Now()

	for key, val := range attrs.StringData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.addAttrsToKeysStatement(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeString, now)

		// 7 columns: unix_milli, tag_key, tag_type, tag_data_type, string_value, int64_value, float64_value
		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeString, val, (*int64)(nil), (*float64)(nil))
	}

	for key, val := range attrs.NumberData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.addAttrsToKeysStatement(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeNumber, now)

		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeNumber, "", (*int64)(nil), &val)
	}

	for key := range attrs.BoolData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.addAttrsToKeysStatement(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeBool, now)

		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeBool, "", (*int64)(nil), (*float64)(nil))
	}
}

func (e *signozAuditExporter) addAttrsToKeysStatement(
	attrKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	key string,
	tagType utils.TagType,
	datatype utils.TagDataType,
	now time.Time,
) {
	if keycheck.IsRandomKey(key) {
		return
	}
	cacheKey := utils.MakeKeyForAttributeKeys(key, tagType, datatype)
	if item := e.keysCache.Get(cacheKey); item != nil {
		return
	}

	// Audit keys tables have 3 columns: name, datatype, timestamp (no DEFAULT on timestamp)
	switch tagType {
	case utils.TagTypeResource:
		_ = resourceKeysStmt.Append(key, string(datatype), now)
	case utils.TagTypeAttribute:
		_ = attrKeysStmt.Append(key, string(datatype), now)
	}
	e.keysCache.Set(cacheKey, struct{}{}, ttlcache.DefaultTTL)
}

func tsBucket(ts int64, bucketSize int64) int64 {
	return (ts / bucketSize) * bucketSize
}

func getOrCreateFingerprint(
	resourcesSeen map[int64]map[string]string,
	bucket int64,
	resourceJSON string,
	res pcommon.Resource,
) string {
	inner, ok := resourcesSeen[bucket]
	if !ok {
		inner = make(map[string]string)
		resourcesSeen[bucket] = inner
	}
	if fp, exists := inner[resourceJSON]; exists {
		return fp
	}
	fp := fingerprint.CalculateFingerprint(res.Attributes().AsRaw(), fingerprint.ResourceHierarchy())
	inner[resourceJSON] = fp
	return fp
}

func attributesToMap(attributes pcommon.Map, forceStringValues bool) attributeMap {
	result := attributeMap{
		StringData: make(map[string]string),
		NumberData: make(map[string]float64),
		BoolData:   make(map[string]bool),
	}
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			result.StringData[k] = v.AsString()
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				result.NumberData[k] = float64(v.Int())
			case pcommon.ValueTypeDouble:
				result.NumberData[k] = v.Double()
			case pcommon.ValueTypeBool:
				result.BoolData[k] = v.Bool()
			default:
				result.StringData[k] = v.AsString()
			}
		}
		return true
	})
	return result
}

func send(statement driver.Batch, tableName string, durationCh chan<- statementSendDuration, chErr chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	start := time.Now()
	err := statement.Send()
	chErr <- err
	durationCh <- statementSendDuration{
		Name:     tableName,
		duration: time.Since(start),
	}
}
