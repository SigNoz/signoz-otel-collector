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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
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
	"go.uber.org/zap"
)

const (
	distributedLogsTable             = "distributed_logs"
	distributedTagAttributes         = "distributed_tag_attributes"
	distributedTagAttributesV2       = "distributed_tag_attributes_v2"
	distributedLogsTableV2           = "distributed_logs_v2"
	logsTableV2                      = "logs_v2"
	distributedLogsResourceV2        = "distributed_logs_v2_resource"
	distributedLogsAttributeKeys     = "distributed_logs_attribute_keys"
	distributedLogsResourceKeys      = "distributed_logs_resource_keys"
	distributedLogsResourceV2Seconds = 1800
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

type shouldSkipKey struct {
	TagKey      string `ch:"tag_key"`
	TagType     string `ch:"tag_type"`
	TagDataType string `ch:"tag_data_type"`
	StringCount uint64 `ch:"string_count"`
	NumberCount uint64 `ch:"number_count"`
}

type attributeMap struct {
	StringData map[string]string
	NumberData map[string]float64
	BoolData   map[string]bool
}

type statementSendDuration struct {
	Name     string
	duration time.Duration
}

type clickhouseLogsExporter struct {
	id              uuid.UUID
	db              clickhouse.Conn
	insertLogsSQLV2 string

	logger *zap.Logger
	cfg    *Config

	usageCollector *usage.UsageCollector

	wg        *sync.WaitGroup
	closeChan chan struct{}

	durationHistogram metric.Float64Histogram

	keysCache *ttlcache.Cache[string, struct{}]
	rfCache   *ttlcache.Cache[string, struct{}]

	shouldSkipKeyValue        atomic.Value // stores map[string]shouldSkipKey
	maxDistinctValues         int
	fetchKeysInterval         time.Duration
	fetchShouldSkipKeysTicker *time.Ticker

	// used to drop logs older than the retention period.
	minAcceptedTs                        atomic.Value
	fetchShouldUpdateMinAcceptedTsTicker *time.Ticker
}

func newExporter(_ exporter.Settings, cfg *Config, opts ...LogExporterOption) (*clickhouseLogsExporter, error) {

	insertLogsSQLV2 := renderInsertLogsSQLV2(cfg)

	// view should be registered after exporter is initialized
	if err := view.Register(LogsCountView, LogsSizeView); err != nil {
		return nil, err
	}

	e := &clickhouseLogsExporter{
		insertLogsSQLV2:   insertLogsSQLV2,
		cfg:               cfg,
		wg:                new(sync.WaitGroup),
		closeChan:         make(chan struct{}),
		maxDistinctValues: cfg.AttributesLimits.MaxDistinctValues,
		fetchKeysInterval: cfg.AttributesLimits.FetchKeysInterval,
	}
	for _, opt := range opts {
		opt(e)
	}

	return e, nil
}

func (e *clickhouseLogsExporter) Start(ctx context.Context, host component.Host) error {
	e.fetchShouldSkipKeysTicker = time.NewTicker(e.fetchKeysInterval)
	e.fetchShouldUpdateMinAcceptedTsTicker = time.NewTicker(10 * time.Minute)

	e.wg.Add(2)

	go func() {
		defer e.wg.Done()
		e.doFetchShouldSkipKeys() // Immediate first fetch
		e.fetchShouldSkipKeys()   // Start ticker routine
	}()
	go func() {
		defer e.wg.Done()
		e.updateMinAcceptedTs()
		e.fetchShouldUpdateMinAcceptedTs()
	}()
	return nil
}

func (e *clickhouseLogsExporter) doFetchShouldSkipKeys() {
	query := fmt.Sprintf(`
		SELECT tag_key, tag_type, tag_data_type, uniq(string_value) as string_count, uniq(number_value) as number_count
		FROM %s.%s
		WHERE unix_milli >= (toUnixTimestamp(now() - toIntervalHour(6)) * 1000)
		GROUP BY tag_key, tag_type, tag_data_type
		HAVING string_count > %d OR number_count > %d
		SETTINGS max_threads = 2`, databaseName, distributedTagAttributesV2, e.maxDistinctValues, e.maxDistinctValues)

	e.logger.Debug("fetching should skip keys", zap.String("query", query))

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
	for {
		select {
		case <-e.closeChan:
			return
		case <-e.fetchShouldSkipKeysTicker.C:
			e.doFetchShouldSkipKeys()
		}
	}
}

// Shutdown will shutdown the exporter.
func (e *clickhouseLogsExporter) Shutdown(_ context.Context) error {
	close(e.closeChan)
	e.wg.Wait()
	if e.fetchShouldSkipKeysTicker != nil {
		e.fetchShouldSkipKeysTicker.Stop()
	}
	if e.fetchShouldUpdateMinAcceptedTsTicker != nil {
		e.fetchShouldUpdateMinAcceptedTsTicker.Stop()
	}
	if e.usageCollector != nil {
		// TODO: handle error
		_ = e.usageCollector.Stop()
	}

	if e.keysCache != nil {
		e.keysCache.Stop()
	}
	if e.rfCache != nil {
		e.rfCache.Stop()
	}

	if e.db != nil {
		err := e.db.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type DBResponseTTL struct {
	EngineFull string `ch:"engine_full"`
}

func (e *clickhouseLogsExporter) updateMinAcceptedTs() {
	e.logger.Info("fetching min accepted ts")

	ctx := context.Background()
	var delTTL uint64 = 0
	var dbResp []DBResponseTTL
	q := fmt.Sprintf("SELECT engine_full FROM system.tables WHERE name='%s' and database='%s'", logsTableV2, databaseName)
	err := e.db.Select(ctx, &dbResp, q)
	if err != nil {
		e.logger.Error("error while fetching ttl", zap.Error(err))
		return
	}
	if len(dbResp) == 0 {
		e.logger.Error("ttl not found")
		return
	}

	deleteTTLExp := regexp.MustCompile(`toIntervalSecond\(([0-9]*)\)`)
	m := deleteTTLExp.FindStringSubmatch(dbResp[0].EngineFull)
	if len(m) > 1 {
		seconds_int, err := strconv.Atoi(m[1])
		if err != nil {
			e.logger.Error("error while converting ttl to int", zap.Error(err))
			return
		}
		delTTL = uint64(seconds_int)
	}

	acceptedDateTime := time.Now().Add(-(time.Duration(delTTL) * time.Second))
	e.minAcceptedTs.Store(uint64(acceptedDateTime.UnixNano()))
}

func (e *clickhouseLogsExporter) fetchShouldUpdateMinAcceptedTs() {
	for {
		select {
		case <-e.closeChan:
			return
		case <-e.fetchShouldUpdateMinAcceptedTsTicker.C:
			e.updateMinAcceptedTs()
		}
	}
}

func (e *clickhouseLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	err := e.pushToClickhouse(ctx, ld)
	if err != nil {
		// we try to remove logs older than the retention period
		// but if we get too many partitions for single INSERT block, we will drop the data
		// StatementSend:code: 252, message: Too many partitions for single INSERT block
		if strings.Contains(err.Error(), "code: 252") {
			e.logger.Warn("too many partitions for single INSERT block, dropping the batch")
			return nil
		}

		return err
	}

	return nil
}

func tsBucket(ts int64, bucketSize int64) int64 {
	return (int64(ts) / int64(bucketSize)) * int64(bucketSize)
}

func (e *clickhouseLogsExporter) pushToClickhouse(ctx context.Context, ld plog.Logs) error {
	oldestAllowedTs := uint64(0)
	if e.minAcceptedTs.Load() != nil {
		oldestAllowedTs = e.minAcceptedTs.Load().(uint64)
	}

	resourcesSeen := map[int64]map[string]string{}

	var insertLogsStmtV2 driver.Batch
	var insertResourcesStmtV2 driver.Batch
	var tagStatementV2 driver.Batch
	var attributeKeysStmt driver.Batch
	var resourceKeysStmt driver.Batch
	var err error

	var shouldSkipKeys map[string]shouldSkipKey
	if e.shouldSkipKeyValue.Load() != nil {
		shouldSkipKeys = e.shouldSkipKeyValue.Load().(map[string]shouldSkipKey)
	}

	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
		start := time.Now()
		chLen := 5

		tagStatementV2, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedTagAttributesV2), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareTagBatchV2:%w", err)
		}
		defer tagStatementV2.Close()

		attributeKeysStmt, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedLogsAttributeKeys), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareAttributeKeysBatch:%w", err)
		}
		defer attributeKeysStmt.Close()

		resourceKeysStmt, err = e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedLogsResourceKeys), driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareResourceKeysBatch:%w", err)
		}
		defer resourceKeysStmt.Close()

		insertLogsStmtV2, err = e.db.PrepareBatch(ctx, e.insertLogsSQLV2, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("PrepareBatchV2:%w", err)
		}
		defer insertLogsStmtV2.Close()

		metrics := map[string]usage.Metric{}

		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resBytes, _ := getResourceAttributesByte(res)
			resourcesMap := attributesToMap(res.Attributes(), true)

			// we are using resourcesMap.StringData here as we are want everything to be string values.
			serializedRes, err := json.Marshal(resourcesMap.StringData)
			if err != nil {
				return fmt.Errorf("couldn't serialize log resource JSON: %w", err)
			}
			resourceJson := string(serializedRes)

			err = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeResource, resourcesMap, shouldSkipKeys)
			if err != nil {
				return err
			}

			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				scope := logs.ScopeLogs().At(j).Scope()
				scopeName := scope.Name()
				scopeVersion := scope.Version()

				scopeMap := attributesToMap(scope.Attributes(), true)

				err := e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeScope, scopeMap, shouldSkipKeys)
				if err != nil {
					return err
				}

				rs := logs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

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

					if ts < oldestAllowedTs {
						e.logger.Debug("skipping log", zap.Uint64("ts", ts), zap.Uint64("oldestAllowedTs", oldestAllowedTs))
						continue
					}

					// generate the id from timestamp
					id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
					if err != nil {
						return fmt.Errorf("IdGenError:%w", err)
					}

					lBucketStart := tsBucket(int64(ts/1000000000), distributedLogsResourceV2Seconds)

					if _, exists := resourcesSeen[int64(lBucketStart)]; !exists {
						resourcesSeen[int64(lBucketStart)] = map[string]string{}
					}
					fp, exists := resourcesSeen[int64(lBucketStart)][resourceJson]
					if !exists {
						fp = fingerprint.CalculateFingerprint(res.Attributes().AsRaw(), fingerprint.ResourceHierarchy())
						resourcesSeen[int64(lBucketStart)][resourceJson] = fp
					}

					attrsMap := attributesToMap(r.Attributes(), false)

					err = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeAttribute, attrsMap, shouldSkipKeys)
					if err != nil {
						return err
					}

					bodyStr := getStringifiedBody(r.Body())
					body, err := flatten.ConvertJSON(bodyStr)
					if err != nil {
						body = bodyStr
					}

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
						body,
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

					// log fields
					logFields := attributeMap{
						StringData: map[string]string{
							"severity_text": r.SeverityText(),
						},
						NumberData: map[string]float64{
							"severity_number": float64(r.SeverityNumber()),
						},
					}
					// TODO: handle error
					_ = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeLogField, logFields, shouldSkipKeys)
				}
			}
		}

		insertResourcesStmtV2, err = e.db.PrepareBatch(
			ctx,
			fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedLogsResourceV2),
			driver.WithReleaseConnection(),
		)
		if err != nil {
			return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
		}
		defer insertResourcesStmtV2.Close()

		for bucketTs, resources := range resourcesSeen {
			for resourceLabels, fingerprint := range resources {
				// if a resource fingerprint is seen in the bucket already, skip inserting it again.
				key := utils.MakeKeyForRFCache(bucketTs, fingerprint)
				if e.rfCache.Get(key) != nil {
					e.logger.Debug("resource fingerprint already present in cache, skipping", zap.String("key", key))
					continue
				}
				// TODO: handle error
				_ = insertResourcesStmtV2.Append(
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
		go send(insertLogsStmtV2, distributedLogsTableV2, chDuration, chErr, &wg)
		go send(insertResourcesStmtV2, distributedLogsResourceV2, chDuration, chErr, &wg)
		go send(attributeKeysStmt, distributedLogsAttributeKeys, chDuration, chErr, &wg)
		go send(resourceKeysStmt, distributedLogsResourceKeys, chDuration, chErr, &wg)
		go send(tagStatementV2, distributedTagAttributesV2, chDuration, chErr, &wg)
		wg.Wait()
		close(chErr)

		// store the duration for send the data
		for i := 0; i < chLen; i++ {
			sendDuration := <-chDuration
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
			// TODO: handle error
			_ = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, e.id.String())}, ExporterSigNozSentLogRecords.M(int64(v.Count)), ExporterSigNozSentLogRecordsBytes.M(int64(v.Size)))
		}

		return err
	}
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
	if keycheck.IsRandomKey(key) {
		return
	}
	cacheKey := utils.MakeKeyForAttributeKeys(key, tagType, datatype)
	// skip if the key is already present
	if item := e.keysCache.Get(cacheKey); item != nil {
		e.logger.Debug("key already present in cache, skipping", zap.String("key", key))
		return
	}

	switch tagType {
	case utils.TagTypeResource:
		// TODO: handle error
		_ = resourceKeysStmt.Append(
			key,
			datatype,
		)
	case utils.TagTypeAttribute:
		// TODO: handle error
		_ = attributeKeysStmt.Append(
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
	attrs attributeMap,
	shouldSkipKeys map[string]shouldSkipKey,
) error {
	unixMilli := (time.Now().UnixMilli() / 3600000) * 3600000
	for attrKey, attrVal := range attrs.StringData {
		if keycheck.IsRandomKey(attrKey) {
			continue
		}
		if len(attrVal) > common.MaxAttributeValueLength {
			e.logger.Debug("attribute value length exceeds the limit", zap.String("key", attrKey))
			continue
		}

		key := utils.MakeKeyForAttributeKeys(attrKey, tagType, utils.TagDataTypeString)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, attrKey, tagType, utils.TagDataTypeString)
		err := tagStatementV2.Append(
			unixMilli,
			attrKey,
			tagType,
			utils.TagDataTypeString,
			attrVal,
			nil,
		)
		if err != nil {
			return fmt.Errorf("could not append string attribute to batch, err: %w", err)
		}
	}

	for numKey, numVal := range attrs.NumberData {
		if keycheck.IsRandomKey(numKey) {
			continue
		}
		key := utils.MakeKeyForAttributeKeys(numKey, tagType, utils.TagDataTypeNumber)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, numKey, tagType, utils.TagDataTypeNumber)
		err := tagStatementV2.Append(
			unixMilli,
			numKey,
			tagType,
			utils.TagDataTypeNumber,
			nil,
			numVal,
		)
		if err != nil {
			return fmt.Errorf("could not append number attribute to batch, err: %w", err)
		}
	}
	for boolKey := range attrs.BoolData {
		if keycheck.IsRandomKey(boolKey) {
			continue
		}
		key := utils.MakeKeyForAttributeKeys(boolKey, tagType, utils.TagDataTypeBool)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}

		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, boolKey, tagType, utils.TagDataTypeBool)
		err := tagStatementV2.Append(
			unixMilli,
			boolKey,
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

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(_ *zap.Logger, cfg *Config) (clickhouse.Conn, error) {
	ctx := context.Background()
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	// setting maxIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	maxIdleConnections := cfg.QueueBatchConfig.NumConsumers + 1
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

func renderInsertLogsSQLV2(_ *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplateV2, databaseName, distributedLogsTableV2)
}
