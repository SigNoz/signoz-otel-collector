// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouselogsexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/segmentio/ksuid"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	DISTRIBUTED_LOGS_TABLE               = "distributed_logs"
	DISTRIBUTED_TAG_ATTRIBUTES           = "distributed_tag_attributes"
	DISTRIBUTED_TAG_ATTRIBUTES_V2        = "distributed_tag_attributes_v2"
	DISTRIBUTED_LOGS_TABLE_V2            = "distributed_logs_v2"
	DISTRIBUTED_LOGS_RESOURCE_V2         = "distributed_logs_v2_resource"
	DISTRIBUTED_LOGS_ATTRIBUTE_KEYS      = "distributed_logs_attribute_keys"
	DISTRIBUTED_LOGS_RESOURCE_KEYS       = "distributed_logs_resource_keys"
	DISTRIBUTED_LOGS_RESOURCE_V2_SECONDS = 1800
)

type shouldSkipKey struct {
	TagKey      string `ch:"tag_key"`
	TagType     string `ch:"tag_type"`
	TagDataType string `ch:"tag_data_type"`
	StringCount uint64 `ch:"string_count"`
	NumberCount uint64 `ch:"number_count"`
}

type clickhouseLogsExporter struct {
	id              uuid.UUID
	db              clickhouse.Conn
	insertLogsSQL   string
	insertLogsSQLV2 string

	logger *zap.Logger
	tracer trace.Tracer
	cfg    *Config

	usageCollector *usage.UsageCollector

	wg        *sync.WaitGroup
	closeChan chan struct{}

	durationHistogram metric.Float64Histogram

	useNewSchema bool

	keysCache *ttlcache.Cache[string, struct{}]
	rfCache   *ttlcache.Cache[string, struct{}]

	shouldSkipKeyValue        atomic.Value // stores map[string]shouldSkipKey
	maxDistinctValues         int
	fetchKeysInterval         time.Duration
	fetchShouldSkipKeysTicker *time.Ticker
}

func newExporter(set exporter.Settings, cfg *Config) (*clickhouseLogsExporter, error) {
	logger := set.Logger.With(zap.String("exporter", "clickhouse_logs"))
	tracer := set.TracerProvider.Tracer("github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter")
	meter := set.MeterProvider.Meter("github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter")

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClickhouseClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	insertLogsSQL := renderInsertLogsSQL(cfg)
	insertLogsSQLV2 := renderInsertLogsSQLV2(cfg)
	id := uuid.New()
	collector := usage.NewUsageCollector(
		id,
		client,
		usage.Options{
			ReportingInterval: usage.DefaultCollectionInterval,
		},
		"signoz_logs",
		UsageExporter,
	)

	collector.Start()

	// view should be registered after exporter is initialized
	if err := view.Register(LogsCountView, LogsSizeView); err != nil {
		return nil, err
	}

	durationHistogram, err := meter.Float64Histogram(
		"exporter_db_write_latency",
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
		return nil, err
	}
	// keys cache is used to avoid duplicate inserts for the same attribute key.
	keysCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](240*time.Minute),
		ttlcache.WithCapacity[string, struct{}](50000),
	)
	go keysCache.Start()

	// resource fingerprint cache is used to avoid duplicate inserts for the same resource fingerprint.
	// the ttl is set to the same as the bucket rounded value i.e 1800 seconds.
	// if a resource fingerprint is seen in the bucket already, skip inserting it again.
	rfCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](DISTRIBUTED_LOGS_RESOURCE_V2_SECONDS*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	return &clickhouseLogsExporter{
		id:                id,
		db:                client,
		insertLogsSQL:     insertLogsSQL,
		insertLogsSQLV2:   insertLogsSQLV2,
		logger:            logger,
		tracer:            tracer,
		cfg:               cfg,
		usageCollector:    collector,
		wg:                new(sync.WaitGroup),
		closeChan:         make(chan struct{}),
		useNewSchema:      cfg.UseNewSchema,
		durationHistogram: durationHistogram,
		keysCache:         keysCache,
		rfCache:           rfCache,
		maxDistinctValues: cfg.AttributesLimits.MaxDistinctValues,
		fetchKeysInterval: cfg.AttributesLimits.FetchKeysInterval,
	}, nil
}

func (e *clickhouseLogsExporter) Start(ctx context.Context, host component.Host) error {
	e.fetchShouldSkipKeysTicker = time.NewTicker(e.fetchKeysInterval)
	go func() {
		e.doFetchShouldSkipKeys() // Immediate first fetch
		e.fetchShouldSkipKeys()   // Start ticker routine
	}()
	return nil
}

func (e *clickhouseLogsExporter) doFetchShouldSkipKeys() {
	query := fmt.Sprintf(`
		SELECT tag_key, tag_type, tag_data_type, countDistinct(string_value) as string_count, countDistinct(number_value) as number_count
		FROM %s.%s
		WHERE unix_milli >= (toUnixTimestamp(now() - toIntervalHour(6)) * 1000)
		GROUP BY tag_key, tag_type, tag_data_type
		HAVING string_count > %d OR number_count > %d
		SETTINGS max_threads = 2`, databaseName, DISTRIBUTED_TAG_ATTRIBUTES_V2, e.maxDistinctValues, e.maxDistinctValues)

	e.logger.Info("fetching should skip keys", zap.String("query", query))

	keys := []shouldSkipKey{}

	err := e.db.Select(context.Background(), &keys, query)
	if err != nil {
		e.logger.Error("error while fetching should skip keys", zap.Error(err))
	}

	shouldSkipKeys := make(map[string]shouldSkipKey)
	for _, key := range keys {
		mapKey := utils.MakeKeyForAttributeKeys(key.TagKey, utils.TagType(key.TagType), utils.TagDataType(key.TagDataType))
		e.logger.Debug("adding to should skip keys", zap.String("key", mapKey), zap.Any("string_count", key.StringCount), zap.Any("number_count", key.NumberCount))
		shouldSkipKeys[mapKey] = key
	}
	e.shouldSkipKeyValue.Store(shouldSkipKeys)
}

func (e *clickhouseLogsExporter) fetchShouldSkipKeys() {
	for range e.fetchShouldSkipKeysTicker.C {
		e.doFetchShouldSkipKeys()
	}
}

// Shutdown will shutdown the exporter.
func (e *clickhouseLogsExporter) Shutdown(_ context.Context) error {
	close(e.closeChan)
	e.wg.Wait()
	if e.fetchShouldSkipKeysTicker != nil {
		e.fetchShouldSkipKeysTicker.Stop()
	}
	if e.usageCollector != nil {
		e.usageCollector.Stop()
	}
	if e.db != nil {
		err := e.db.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *clickhouseLogsExporter) getLogsTTLSeconds(ctx context.Context) (int, error) {
	type DBResponseTTL struct {
		EngineFull string `ch:"engine_full"`
	}

	var delTTL int = -1

	var dbResp []DBResponseTTL
	q := fmt.Sprintf("SELECT engine_full FROM system.tables WHERE name='%s' and database='%s'", tableName, databaseName)
	err := e.db.Select(ctx, &dbResp, q)
	if err != nil {
		return delTTL, err
	}
	if len(dbResp) == 0 {
		return delTTL, fmt.Errorf("ttl not found")
	}

	deleteTTLExp := regexp.MustCompile(`toIntervalSecond\(([0-9]*)\)`)

	m := deleteTTLExp.FindStringSubmatch(dbResp[0].EngineFull)
	if len(m) > 1 {
		seconds_int, err := strconv.Atoi(m[1])
		if err != nil {
			return delTTL, nil
		}
		delTTL = seconds_int
	}

	return delTTL, nil
}

func (e *clickhouseLogsExporter) removeOldLogs(ctx context.Context, ld plog.Logs) error {
	// get the TTL.
	ttL, err := e.getLogsTTLSeconds(ctx)
	if err != nil {
		return err
	}

	// if logs contains timestamp before acceptedDateTime, it will be rejected
	acceptedDateTime := time.Now().Add(-(time.Duration(ttL) * time.Second))

	removeLog := func(log plog.LogRecord) bool {
		t := log.Timestamp().AsTime()
		return t.Unix() < acceptedDateTime.Unix()
	}
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			logs.ScopeLogs().At(j).LogRecords().RemoveIf(removeLog)
		}
	}
	return nil
}

func (e *clickhouseLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	for i := 0; i <= 1; i++ {
		err := e.pushToClickhouse(ctx, ld)
		if err != nil {
			// StatementSend:code: 252, message: Too many partitions for single INSERT block
			// iterating twice since we want to try once after removing the old data
			if i == 1 || !strings.Contains(err.Error(), "code: 252") {
				// TODO(nitya): after returning it will be retried, ideally it should be pushed to DLQ
				return err
			}

			// drop logs older than TTL
			removeLogsError := e.removeOldLogs(ctx, ld)
			if removeLogsError != nil {
				return fmt.Errorf("error while dropping old logs %v, after error %v", removeLogsError, err)
			}
		} else {
			break
		}
	}

	return nil
}

func tsBucket(ts int64, bucketSize int64) int64 {
	return (int64(ts) / int64(bucketSize)) * int64(bucketSize)
}

func (e *clickhouseLogsExporter) pushToClickhouse(ctx context.Context, ld plog.Logs) error {
	ctx, span := e.tracer.Start(ctx, "pushToClickhouse")
	defer span.End()

	resourcesSeen := map[int64]map[string]string{}

	var insertLogsStmtV2 driver.Batch
	var insertResourcesStmtV2 driver.Batch
	var statement driver.Batch
	var tagStatementV2 driver.Batch
	var attributeKeysStmt driver.Batch
	var resourceKeysStmt driver.Batch
	var err error

	var shouldSkipKeys map[string]shouldSkipKey
	if e.shouldSkipKeyValue.Load() != nil {
		shouldSkipKeys = e.shouldSkipKeyValue.Load().(map[string]shouldSkipKey)
	}

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
		if insertLogsStmtV2 != nil {
			_ = insertLogsStmtV2.Abort()
		}
		if insertResourcesStmtV2 != nil {
			_ = insertResourcesStmtV2.Abort()
		}
		if attributeKeysStmt != nil {
			_ = attributeKeysStmt.Abort()
		}
		if resourceKeysStmt != nil {
			_ = resourceKeysStmt.Abort()
		}
	}()

	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
		start := time.Now()
		chLen := 5
		if !e.useNewSchema {
			chLen = 6
			statement, err = e.db.PrepareBatch(ctx, e.insertLogsSQL, driver.WithReleaseConnection())
			if err != nil {
				return fmt.Errorf("PrepareBatch:%w", err)
			}
		}

		tagStatementV2, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, DISTRIBUTED_TAG_ATTRIBUTES_V2), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareTagBatchV2:%w", err)
		}

		attributeKeysStmt, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, DISTRIBUTED_LOGS_ATTRIBUTE_KEYS), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareAttributeKeysBatch:%w", err)
		}

		resourceKeysStmt, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, DISTRIBUTED_LOGS_RESOURCE_KEYS), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareResourceKeysBatch:%w", err)
		}

		insertLogsStmtV2, err = e.db.PrepareBatch(ctx, e.insertLogsSQLV2, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareBatchV2:%w", err)
		}

		metrics := map[string]usage.Metric{}

		var resourceAttributesDuration time.Duration
		var logAttributesDuration time.Duration
		var addToTagStatementDuration time.Duration
		var addToLogStatementDuration time.Duration

		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resStart := time.Now()
			resBytes, _ := json.Marshal(res.Attributes().AsRaw())

			resources := attributesToSlice(res.Attributes(), true)

			resourcesMap := attributesToMap(res.Attributes(), true)

			// we are using resourcesMap.StringData here as we are want everything to be string values.
			serializedRes, err := json.Marshal(resourcesMap.StringData)
			if err != nil {
				return fmt.Errorf("couldn't serialize log resource JSON: %w", err)
			}
			resourceJson := string(serializedRes)
			resourceAttributesDuration += time.Since(resStart)

			addToTagStatementStart := time.Now()
			err = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeResource, resources, e.useNewSchema, shouldSkipKeys)
			if err != nil {
				return err
			}
			addToTagStatementDuration += time.Since(addToTagStatementStart)

			// remove after sometime
			resources = addTemporaryUnderscoreSupport(resources)

			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				scope := logs.ScopeLogs().At(j).Scope()
				scopeName := scope.Name()
				scopeVersion := scope.Version()

				scopeAttributes := attributesToSlice(scope.Attributes(), true)
				scopeMap := attributesToMap(scope.Attributes(), true)

				err := e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeScope, scopeAttributes, e.useNewSchema, shouldSkipKeys)
				if err != nil {
					return err
				}

				rs := logs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

					logStart := time.Now()
					// capturing the metrics
					tenant := usage.GetTenantNameFromResource(logs.Resource())
					attrBytes, _ := json.Marshal(r.Attributes().AsRaw())
					usage.AddMetric(metrics, tenant, 1, int64(len([]byte(r.Body().AsString()))+len(attrBytes)+len(resBytes)))

					// set observedTimestamp as the default timestamp if timestamp is empty.
					ts := uint64(r.Timestamp())
					ots := uint64(r.ObservedTimestamp())
					if ots == 0 {
						ots = uint64(time.Now().UnixNano())
					}
					if ts == 0 {
						ts = ots
					}

					// generate the id from timestamp
					id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
					if err != nil {
						return fmt.Errorf("IdGenError:%w", err)
					}

					lBucketStart := tsBucket(int64(ts/1000000000), DISTRIBUTED_LOGS_RESOURCE_V2_SECONDS)

					if _, exists := resourcesSeen[int64(lBucketStart)]; !exists {
						resourcesSeen[int64(lBucketStart)] = map[string]string{}
					}
					fp, exists := resourcesSeen[int64(lBucketStart)][resourceJson]
					if !exists {
						fp = fingerprint.CalculateFingerprint(res.Attributes().AsRaw(), fingerprint.ResourceHierarchy())
						resourcesSeen[int64(lBucketStart)][resourceJson] = fp
					}

					attributes := attributesToSlice(r.Attributes(), false)
					attrsMap := attributesToMap(r.Attributes(), false)
					logAttributesDuration += time.Since(logStart)

					addToTagStatementStart = time.Now()
					err = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeAttribute, attributes, e.useNewSchema, shouldSkipKeys)
					if err != nil {
						return err
					}
					addToTagStatementDuration += time.Since(addToTagStatementStart)

					insertLogsStart := time.Now()
					err = insertLogsStmtV2.Append(
						uint64(lBucketStart),
						fp,
						ts,
						ots,
						id.String(),
						utils.TraceIDToHexOrEmptyString(r.TraceID()),
						utils.SpanIDToHexOrEmptyString(r.SpanID()),
						uint32(r.Flags()),
						r.SeverityText(),
						uint8(r.SeverityNumber()),
						getStringifiedBody(r.Body()),
						attrsMap.StringData,
						attrsMap.NumberData,
						attrsMap.BoolData,
						resourcesMap.StringData,
						scopeName,
						scopeVersion,
						scopeMap.StringData,
					)
					if err != nil {
						return fmt.Errorf("StatementAppendLogsV2:%w", err)
					}
					addToLogStatementDuration += time.Since(insertLogsStart)
					// old table
					if !e.useNewSchema {
						// remove after sometime
						attributes = addTemporaryUnderscoreSupport(attributes)
						err = statement.Append(
							ts,
							ots,
							id.String(),
							utils.TraceIDToHexOrEmptyString(r.TraceID()),
							utils.SpanIDToHexOrEmptyString(r.SpanID()),
							uint32(r.Flags()),
							r.SeverityText(),
							uint8(r.SeverityNumber()),
							getStringifiedBody(r.Body()),
							resources.StringKeys,
							resources.StringValues,
							attributes.StringKeys,
							attributes.StringValues,
							attributes.IntKeys,
							attributes.IntValues,
							attributes.FloatKeys,
							attributes.FloatValues,
							attributes.BoolKeys,
							attributes.BoolValues,
							scopeName,
							scopeVersion,
							scopeAttributes.StringKeys,
							scopeAttributes.StringValues,
						)
					}
					if err != nil {
						return fmt.Errorf("StatementAppend:%w", err)
					}
				}
			}
		}
		span.SetAttributes(
			attribute.Int64("resource_attributes_processing_total_time", resourceAttributesDuration.Milliseconds()),
			attribute.Int64("log_attributes_processing_total_time", logAttributesDuration.Milliseconds()),
			attribute.Int64("add_to_tag_statement_total_time", addToTagStatementDuration.Milliseconds()),
			attribute.Int64("add_to_log_statement_total_time", addToLogStatementDuration.Milliseconds()),
			attribute.Int64("logs_count", int64(ld.LogRecordCount())),
		)

		insertResourcesStmtV2, err = e.db.PrepareBatch(
			ctx,
			fmt.Sprintf("INSERT into %s.%s", databaseName, DISTRIBUTED_LOGS_RESOURCE_V2),
			driver.WithReleaseConnection(),
		)
		if err != nil {
			return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
		}

		for bucketTs, resources := range resourcesSeen {
			for resourceLabels, fingerprint := range resources {
				// if a resource fingerprint is seen in the bucket already, skip inserting it again.
				key := utils.MakeKeyForRFCache(bucketTs, fingerprint)
				if e.rfCache.Get(key) != nil {
					e.logger.Debug("resource fingerprint already present in cache, skipping", zap.String("key", key))
					continue
				}
				insertResourcesStmtV2.Append(
					resourceLabels,
					fingerprint,
					bucketTs,
				)
				e.rfCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
			}
		}

		var wg sync.WaitGroup
		chErr := make(chan error, chLen)
		chDuration := make(chan statementSendDuration, chLen)

		wg.Add(chLen)
		if !e.useNewSchema {
			go send(statement, DISTRIBUTED_LOGS_TABLE, chDuration, chErr, &wg)
		}
		go send(insertLogsStmtV2, DISTRIBUTED_LOGS_TABLE_V2, chDuration, chErr, &wg)
		go send(insertResourcesStmtV2, DISTRIBUTED_LOGS_RESOURCE_V2, chDuration, chErr, &wg)
		go send(attributeKeysStmt, DISTRIBUTED_LOGS_ATTRIBUTE_KEYS, chDuration, chErr, &wg)
		go send(resourceKeysStmt, DISTRIBUTED_LOGS_RESOURCE_KEYS, chDuration, chErr, &wg)
		go send(tagStatementV2, DISTRIBUTED_TAG_ATTRIBUTES_V2, chDuration, chErr, &wg)
		wg.Wait()
		close(chErr)

		// store the duration for send the data
		for i := 0; i < chLen; i++ {
			sendDuration := <-chDuration
			span.SetAttributes(attribute.Int64("send_"+sendDuration.Name+"_time", sendDuration.duration.Milliseconds()))
			e.durationHistogram.Record(
				ctx,
				float64(sendDuration.duration.Milliseconds()),
				metric.WithAttributes(
					attribute.String("table", sendDuration.Name),
					attribute.String("exporter", pipeline.SignalLogs.String()),
				),
			)
		}

		// check the errors
		for i := 0; i < chLen; i++ {
			if r := <-chErr; r != nil {
				return fmt.Errorf("StatementSend:%w", r)
			}
		}

		duration := time.Since(start)
		e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
			zap.String("cost", duration.String()))

		for k, v := range metrics {
			stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, e.id.String())}, ExporterSigNozSentLogRecords.M(int64(v.Count)), ExporterSigNozSentLogRecordsBytes.M(int64(v.Size)))
		}

		return err
	}
}

type statementSendDuration struct {
	Name     string
	duration time.Duration
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

type attributesToSliceResponse struct {
	StringKeys   []string
	StringValues []string
	IntKeys      []string
	IntValues    []int64
	FloatKeys    []string
	FloatValues  []float64
	BoolKeys     []string
	BoolValues   []bool
}

func getStringifiedBody(body pcommon.Value) string {
	var strBody string
	switch body.Type() {
	case pcommon.ValueTypeBytes:
		strBody = string(body.Bytes().AsRaw())
	default:
		strBody = body.AsString()
	}
	return strBody
}

func (e *clickhouseLogsExporter) addAttrsToAttributeKeysStatement(
	attributeKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	key string,
	tagType utils.TagType,
	datatype utils.TagDataType,
) {
	cacheKey := utils.MakeKeyForAttributeKeys(key, tagType, datatype)
	// skip if the key is already present
	if item := e.keysCache.Get(cacheKey); item != nil {
		e.logger.Debug("key already present in cache, skipping", zap.String("key", key))
		return
	}

	switch tagType {
	case utils.TagTypeResource:
		resourceKeysStmt.Append(
			key,
			datatype,
		)
	case utils.TagTypeAttribute:
		attributeKeysStmt.Append(
			key,
			datatype,
		)
	}
	e.keysCache.Set(cacheKey, struct{}{}, ttlcache.DefaultTTL)
}

func (e *clickhouseLogsExporter) addAttrsToTagStatement(
	tagStatementV2 driver.Batch,
	attributeKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	tagType utils.TagType,
	attrs attributesToSliceResponse,
	_ bool,
	shouldSkipKeys map[string]shouldSkipKey,
) error {
	unixMilli := (time.Now().UnixMilli() / 3600000) * 3600000
	for i, v := range attrs.StringKeys {

		if len(attrs.StringValues[i]) > common.MaxAttributeValueLength {
			e.logger.Debug("attribute value length exceeds the limit", zap.String("key", v))
			continue
		}

		key := utils.MakeKeyForAttributeKeys(v, tagType, utils.TagDataTypeString)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, v, tagType, utils.TagDataTypeString)
		err := tagStatementV2.Append(
			unixMilli,
			v,
			tagType,
			utils.TagDataTypeString,
			attrs.StringValues[i],
			nil,
		)
		if err != nil {
			return fmt.Errorf("could not append string attribute to batch, err: %w", err)
		}
	}

	for i, v := range attrs.IntKeys {
		key := utils.MakeKeyForAttributeKeys(v, tagType, utils.TagDataTypeNumber)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, v, tagType, utils.TagDataTypeNumber)
		err := tagStatementV2.Append(
			unixMilli,
			v,
			tagType,
			utils.TagDataTypeNumber,
			nil,
			attrs.IntValues[i],
		)
		if err != nil {
			return fmt.Errorf("could not append number attribute to batch, err: %w", err)
		}
	}
	for i, v := range attrs.FloatKeys {
		key := utils.MakeKeyForAttributeKeys(v, tagType, utils.TagDataTypeNumber)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, v, tagType, utils.TagDataTypeNumber)
		err := tagStatementV2.Append(
			unixMilli,
			v,
			tagType,
			utils.TagDataTypeNumber,
			nil,
			attrs.FloatValues[i],
		)
		if err != nil {
			return fmt.Errorf("could not append number attribute to batch, err: %w", err)
		}
	}
	for _, v := range attrs.BoolKeys {
		key := utils.MakeKeyForAttributeKeys(v, tagType, utils.TagDataTypeBool)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}

		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, v, tagType, utils.TagDataTypeBool)
		err := tagStatementV2.Append(
			unixMilli,
			v,
			tagType,
			utils.TagDataTypeBool,
			nil,
			nil,
		)
		if err != nil {
			return fmt.Errorf("could not append bool attribute to batch, err: %w", err)
		}
	}
	return nil
}

type attributeMap struct {
	StringData map[string]string
	NumberData map[string]float64
	BoolData   map[string]bool
}

func attributesToMap(attributes pcommon.Map, forceStringValues bool) (response attributeMap) {
	response.BoolData = map[string]bool{}
	response.StringData = map[string]string{}
	response.NumberData = map[string]float64{}
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			// store everything as string
			response.StringData[k] = v.AsString()
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				response.NumberData[k] = float64(v.Int())
			case pcommon.ValueTypeDouble:
				response.NumberData[k] = v.Double()
			case pcommon.ValueTypeBool:
				response.BoolData[k] = v.Bool()
			default: // store it as string
				response.StringData[k] = v.AsString()
			}
		}

		return true
	})
	return response
}

func attributesToSlice(attributes pcommon.Map, forceStringValues bool) (response attributesToSliceResponse) {
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			// store everything as string
			response.StringKeys = append(response.StringKeys, k)
			response.StringValues = append(response.StringValues, v.AsString())
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				response.IntKeys = append(response.IntKeys, k)
				response.IntValues = append(response.IntValues, v.Int())
			case pcommon.ValueTypeDouble:
				response.FloatKeys = append(response.FloatKeys, k)
				response.FloatValues = append(response.FloatValues, v.Double())
			case pcommon.ValueTypeBool:
				response.BoolKeys = append(response.BoolKeys, k)
				response.BoolValues = append(response.BoolValues, v.Bool())
			default: // store it as string
				response.StringKeys = append(response.StringKeys, k)
				response.StringValues = append(response.StringValues, v.AsString())
			}
		}

		return true
	})
	return response
}

// remove this function after sometime.
func addTemporaryUnderscoreSupport(data attributesToSliceResponse) attributesToSliceResponse {
	for i, v := range data.BoolKeys {
		if strings.Contains(v, ".") {
			data.BoolKeys = append(data.BoolKeys, formatKey(v))
			data.BoolValues = append(data.BoolValues, data.BoolValues[i])
		}
	}

	for i, v := range data.StringKeys {
		if strings.Contains(v, ".") {
			data.StringKeys = append(data.StringKeys, formatKey(v))
			data.StringValues = append(data.StringValues, data.StringValues[i])
		}
	}
	for i, v := range data.IntKeys {
		if strings.Contains(v, ".") {
			data.IntKeys = append(data.IntKeys, formatKey(v))
			data.IntValues = append(data.IntValues, data.IntValues[i])
		}
	}
	for i, v := range data.FloatKeys {
		if strings.Contains(v, ".") {
			data.FloatKeys = append(data.FloatKeys, formatKey(v))
			data.FloatValues = append(data.FloatValues, data.FloatValues[i])
		}
	}

	return data
}

func formatKey(k string) string {
	return strings.ReplaceAll(k, ".", "_")
}

const (
	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s.%s (
							timestamp,
							observed_timestamp,
							id,
							trace_id,
							span_id,
							trace_flags,
							severity_text,
							severity_number,
							body,
							resources_string_key,
							resources_string_value,
							attributes_string_key,
							attributes_string_value,
							attributes_int64_key,
							attributes_int64_value,
							attributes_float64_key,
							attributes_float64_value,
							attributes_bool_key,
							attributes_bool_value,
							scope_name,
							scope_version,
							scope_string_key,
							scope_string_value
							) VALUES (
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								)`
)

const (
	// language=ClickHouse SQL
	insertLogsSQLTemplateV2 = `INSERT INTO %s.%s (
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
							attributes_string,
							attributes_number,
							attributes_bool,
							resources_string,
							scope_name,
							scope_version,
							scope_string
							) VALUES (
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?,
								?
								)`
)

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(_ *zap.Logger, cfg *Config) (clickhouse.Conn, error) {
	// use empty database to create database
	ctx := context.Background()
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	// setting maxIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	maxIdleConnections := cfg.QueueConfig.NumConsumers + 1
	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func renderInsertLogsSQL(_ *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, databaseName, DISTRIBUTED_LOGS_TABLE)
}

func renderInsertLogsSQLV2(_ *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplateV2, databaseName, DISTRIBUTED_LOGS_TABLE_V2)
}
