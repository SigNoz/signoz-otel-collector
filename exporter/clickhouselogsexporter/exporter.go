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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/sync/errgroup"

	"github.com/ClickHouse/clickhouse-go/v2"
	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
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
	"go.uber.org/zap"
)

const (
	distributedTagAttributesV2       = "distributed_tag_attributes_v2"
	distributedLogsTableV2           = "distributed_logs_v2"
	logsTableV2                      = "logs_v2"
	distributedLogsResourceV2        = "distributed_logs_v2_resource"
	distributedLogsAttributeKeys     = "distributed_logs_attribute_keys"
	distributedLogsResourceKeys      = "distributed_logs_resource_keys"
	distributedColumnEvolutionTable  = constants.SignozMetadataDB + ".distributed_column_evolution_metadata"
	distributedLogsResourceV2Seconds = 1800
	// language=ClickHouse SQL
	insertLogsResourceSQLTemplate = `INSERT INTO %s.%s (
		labels,
		fingerprint,
		seen_at_ts_bucket_start
		) VALUES (
			?,
			?,
			?
	)`
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
		resource,
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
			?,
			?
			)`
	insertLogsSQLTemplateV2WithBodyJSON = `INSERT INTO %s.%s (
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
		body_json,
		body_json_promoted,
		attributes_string,
		attributes_number,
		attributes_bool,
		resources_string,
		resource,
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

// Record represents a prepared log record, ready to be appended to ClickHouse batches.
type Record struct {
	// batch columns
	tsBucketStart    uint64
	resourceFP       string
	ts               uint64
	ots              uint64
	id               string
	traceID          string
	spanID           string
	traceFlags       uint32
	severityText     string
	severityNum      uint8
	body             string
	bodyJSON         string
	bodyJSONPromoted string
	scopeName        string
	scopeVersion     string
	// attribute/tag maps to be appended by the single consumer
	resourceMap attributeMap
	scopeMap    attributeMap
	attrsMap    attributeMap
	logFields   attributeMap
	recordSize  int64
}

type statementSendDuration struct {
	Name     string
	duration time.Duration
}

// resourcesSeenMap is a thread-safe map for storing resource fingerprints by bucket timestamp.
// Uses nested sync.Map for lock-free concurrent access: outer map keyed by bucket (int64),
// inner map keyed by resource JSON (string) with fingerprint (string) as value.
type resourcesSeenMap struct {
	buckets sync.Map // map[int64]*sync.Map[string, string]
}

// newResourcesSeenMap creates a new thread-safe resourcesSeenMap.
func newResourcesSeenMap() *resourcesSeenMap {
	return &resourcesSeenMap{}
}

// getOrCreateFingerprint returns the fingerprint for the given bucket and resource JSON.
// If it doesn't exist, it creates and stores a new fingerprint.
// Uses nested sync.Map for lock-free concurrent access to different buckets.
func (r *resourcesSeenMap) getOrCreateFingerprint(bucket int64, resourceJSON string, createFn func() string) string {
	// Get or create the inner map for this bucket
	bucketVal, _ := r.buckets.LoadOrStore(bucket, &sync.Map{})
	innerMap := bucketVal.(*sync.Map)

	// check if fingerprint already exists
	if fpVal, exists := innerMap.Load(resourceJSON); exists {
		return fpVal.(string)
	}

	//  need to create fingerprint
	// Use LoadOrStore to handle race condition where another goroutine might create it
	fp := createFn()
	actualVal, _ := innerMap.LoadOrStore(resourceJSON, fp)
	return actualVal.(string)
}

// rangeAll iterates over all entries in the map, calling fn for each resourceKey and fingerprintVal.
// Errors are handled immediately and iteration stops on first error.
func (r *resourcesSeenMap) rangeAll(fn func(bucketTs int64, resourceKey, fingerprintVal string) error) error {
	var err error
	r.buckets.Range(func(key, value interface{}) bool {
		// to break the iteration
		if err != nil {
			return false
		}
		bucketTs, yes := key.(int64)
		if !yes {
			err = fmt.Errorf("expected bucketTs to be int64, found %T", key)
			return false
		}
		innerMap, yes := value.(*sync.Map)
		if !yes {
			err = fmt.Errorf("expected bucket value to be *sync.Map, found %T", value)
			return false
		}

		innerMap.Range(func(resourceKey, fingerprintVal interface{}) bool {
			resourceLabels, yes := resourceKey.(string)
			if !yes {
				err = fmt.Errorf("expected resourceLables to be string, found %T", resourceKey)
				return false
			}
			fingerprint, yes := fingerprintVal.(string)
			if !yes {
				err = fmt.Errorf("expected fingerprint to be string, found %T", fingerprintVal)
				return false
			}

			if err = fn(bucketTs, resourceLabels, fingerprint); err != nil {
				return false
			}
			return true
		})
		return true
	})
	return err
}

type clickhouseLogsExporter struct {
	id                     uuid.UUID
	db                     clickhouse.Conn
	insertLogsSQLV2        string
	insertLogsResourceSQL  string
	bodyJSONEnabled        bool
	bodyJSONOldBodyEnabled bool

	logger *zap.Logger
	cfg    *Config

	usageCollector *usage.UsageCollector

	limiter chan struct{}

	wg        *sync.WaitGroup
	closeChan chan struct{}

	durationHistogram metric.Float64Histogram

	keysCache *ttlcache.Cache[string, struct{}]
	rfCache   *ttlcache.Cache[string, struct{}]

	shouldSkipKeyValue    atomic.Value // stores map[string]shouldSkipKey
	maxDistinctValues     int
	fetchKeysInterval     time.Duration
	shutdownFuncs         []func() error
	maxAllowedDataAgeDays int

	// promotedPaths holds a set of JSON paths that should be promoted.
	// Accessed via atomic.Value to allow lock-free reads on hot path.
	promotedPaths             atomic.Value // stores map[string]struct{}
	promotedPathsSyncInterval time.Duration
}

func newExporter(_ exporter.Settings, cfg *Config, opts ...LogExporterOption) (*clickhouseLogsExporter, error) {
	// view should be registered after exporter is initialized
	if err := view.Register(LogsCountView, LogsSizeView); err != nil {
		return nil, err
	}

	maxAllowedDataAgeDays := defaultMaxAllowedDataAgeDays
	if cfg.MaxAllowedDataAgeDays != nil {
		maxAllowedDataAgeDays = *cfg.MaxAllowedDataAgeDays
	}

	e := &clickhouseLogsExporter{
		insertLogsSQLV2:           renderInsertLogsSQLV2(cfg.BodyJSONEnabled),
		insertLogsResourceSQL:     renderInsertLogsResourceSQL(cfg),
		cfg:                       cfg,
		bodyJSONEnabled:           cfg.BodyJSONEnabled,
		wg:                        new(sync.WaitGroup),
		closeChan:                 make(chan struct{}),
		maxDistinctValues:         cfg.AttributesLimits.MaxDistinctValues,
		fetchKeysInterval:         cfg.AttributesLimits.FetchKeysInterval,
		promotedPathsSyncInterval: *cfg.PromotedPathsSyncInterval,
		bodyJSONOldBodyEnabled:    cfg.BodyJSONOldBodyEnabled,
		limiter:                   make(chan struct{}, utils.Concurrency()),
		maxAllowedDataAgeDays:     maxAllowedDataAgeDays,
	}
	for _, opt := range opts {
		opt(e)
	}

	// Ensure promotedPaths is always initialized so reads and type assertions are safe
	e.promotedPaths.Store(map[string]struct{}{})

	return e, nil
}

func (e *clickhouseLogsExporter) Start(ctx context.Context, host component.Host) error {
	e.fetchShouldSkipKeys() // Start ticker routine
	e.fetchPromotedPaths()
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
	ticker := time.NewTicker(e.fetchKeysInterval)
	e.shutdownFuncs = append(e.shutdownFuncs, func() error {
		ticker.Stop()
		return nil
	})

	e.doFetchShouldSkipKeys() // Immediate first fetch
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-e.closeChan:
				return
			case <-ticker.C:
				e.doFetchShouldSkipKeys()
			}
		}
	}()
}

// fetchPromotedPaths periodically loads promoted JSON paths from ClickHouse into memory.
func (e *clickhouseLogsExporter) fetchPromotedPaths() {
	// if body JSON columns are activated, fetch promoted paths periodically
	if e.bodyJSONEnabled {
		ticker := time.NewTicker(e.promotedPathsSyncInterval)
		e.shutdownFuncs = append(e.shutdownFuncs, func() error {
			ticker.Stop()
			return nil
		})

		e.doFetchPromotedPaths() // Immediate first fetch
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for {
				select {
				case <-e.closeChan:
					return
				case <-ticker.C:
					e.doFetchPromotedPaths()
				}
			}
		}()
	}
}

func (e *clickhouseLogsExporter) doFetchPromotedPaths() {
	// Query Evolution Table for promoted paths
	// Format: signal, col_name, col_type, field_context, field_name, release_time
	// Example: logs, body_json_promoted, JSON, body, user.name, Jan 10
	query := fmt.Sprintf(
		`SELECT field_name FROM %s WHERE signal = 'logs' AND column_name = '%s' AND field_context = 'body' AND field_name != '__all__' SETTINGS max_threads = 1`,
		distributedColumnEvolutionTable,
		constants.BodyPromotedColumn,
	)
	e.logger.Debug("fetching promoted paths from evolution table", zap.String("query", query))

	rows := []struct {
		FieldName string `ch:"field_name"`
	}{}
	if err := e.db.Select(context.Background(), &rows, query); err != nil {
		e.logger.Error("error while fetching promoted paths from evolution table", zap.Error(err))
		return
	}
	updated := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		updated[r.FieldName] = struct{}{}
	}

	e.promotedPaths.Store(updated)
}

// Shutdown will shutdown the exporter.
func (e *clickhouseLogsExporter) Shutdown(_ context.Context) error {
	close(e.closeChan)
	e.wg.Wait()
	for _, shutdownFunc := range e.shutdownFuncs {
		if err := shutdownFunc(); err != nil {
			return err
		}
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
	oldestAllowedTs := uint64(time.Now().Add(-time.Duration(e.maxAllowedDataAgeDays) * 24 * time.Hour).UnixNano())

	start := time.Now()
	chLen := 5

	var insertLogsStmtV2 driver.Batch
	var insertResourcesStmtV2 driver.Batch
	var tagStatementV2 driver.Batch
	var attributeKeysStmt driver.Batch
	var resourceKeysStmt driver.Batch

	var shouldSkipKeys map[string]shouldSkipKey
	if e.shouldSkipKeyValue.Load() != nil {
		shouldSkipKeys = e.shouldSkipKeyValue.Load().(map[string]shouldSkipKey)
	}

	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
	}

	// Prepare batches first
	tagStatementV2, err := e.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", databaseName, distributedTagAttributesV2), driver.WithReleaseConnection())
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

	// resource fingerprints aggregated by consumer
	resourcesSeen := newResourcesSeenMap()
	metrics := map[string]usage.Metric{}

	// records channel and limiter
	recordStream := make(chan *Record, cap(e.limiter))

	// ensure recordStream is closed in all scenarios (producer completes or errgroup cancels)
	// Case 1: Producer completes successfully; consumer will not exit until channel is closed
	//  and hence closing the channel is required to allow the consumer to exit else code will be
	//  stuck at group.Wait() waiting forever for consumer to exit when the last batch of concurrent logs have been sent in the channel
	// Case 2: Error occurr in either producer or consumer; both will exit with the help of groupCtx.Done()
	//  and channel will get closed by after waiting for producerWG
	// Case 3: Error occurred in producer loop, outside the errgroup, defer closeRecordStream() will be called
	//  for closing the channel
	var closeOnce sync.Once
	closeRecordStream := func() { closeOnce.Do(func() { close(recordStream) }) }
	defer closeRecordStream()
	// wait for all producer goroutines to finish sending before closing channel
	// if Channel closed before all producers finish, we will get a panic of send on closed channel
	var producersWG sync.WaitGroup

	// Why SetLimit(utils.Concurrency()) is not used?
	// because we want to limit the number of producers but the Consumer is also part of same errgroup
	group, groupCtx := errgroup.WithContext(ctx)

	// consumer: Append to batches and aggregate metrics
	group.Go(func() error {
		for {
			select {
			case <-groupCtx.Done(): // process cancelled by producer error
				return nil
			case rec, open := <-recordStream:
				if !open {
					return nil
				}
				// tags for resource/scope/attrs
				if err := e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeResource, rec.resourceMap, shouldSkipKeys); err != nil {
					return err
				}
				if err := e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeScope, rec.scopeMap, shouldSkipKeys); err != nil {
					return err
				}
				if err := e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeAttribute, rec.attrsMap, shouldSkipKeys); err != nil {
					return err
				}
				// log fields
				_ = e.addAttrsToTagStatement(tagStatementV2, attributeKeysStmt, resourceKeysStmt, utils.TagTypeLogField, rec.logFields, shouldSkipKeys)

				// append main log row
				args := []any{
					rec.tsBucketStart,
					rec.resourceFP,
					rec.ts,
					rec.ots,
					rec.id,
					rec.traceID,
					rec.spanID,
					rec.traceFlags,
					rec.severityText,
					rec.severityNum,
					rec.body,
				}
				if e.bodyJSONEnabled {
					args = append(args, rec.bodyJSON, rec.bodyJSONPromoted)
				}
				args = append(args,
					rec.attrsMap.StringData,
					rec.attrsMap.NumberData,
					rec.attrsMap.BoolData,
					rec.resourceMap.StringData,
					rec.resourceMap.StringData,
					rec.scopeName,
					rec.scopeVersion,
					rec.scopeMap.StringData,
				)
				if err := insertLogsStmtV2.Append(args...); err != nil {
					return fmt.Errorf("StatementAppendLogsV2:%w", err)
				}

				// aggregate metrics
				tenant := usage.GetTenantNameFromResource()
				usage.AddMetric(metrics, tenant, 1, rec.recordSize)
			}
		}
	})

	// Producer: iterate logs and spawn workers limited by e.limiter
producerIteration:
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		res := logs.Resource()
		resBytes, _ := getResourceAttributesByte(res)
		resourcesMap := attributesToMap(res.Attributes(), true)
		serializedRes, err := json.Marshal(resourcesMap.StringData)
		if err != nil {
			return fmt.Errorf("couldn't serialize log resource JSON: %w", err)
		}
		resourceJson := string(serializedRes)

		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			scope := logs.ScopeLogs().At(j).Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeMap := attributesToMap(scope.Attributes(), true)

			records := logs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				record := records.At(k)

				// block the execution until we acquire limiter slot
				select {
				case <-groupCtx.Done():
					break producerIteration // immidiately break the producer loop and reach group.Wait()
				case e.limiter <- struct{}{}:
				}

				producersWG.Add(1)
				group.Go(func() error {
					defer func() { <-e.limiter; producersWG.Done() }()

					// timestamps
					ts := uint64(record.Timestamp())
					ots := uint64(record.ObservedTimestamp())
					if ots == 0 {
						ots = uint64(time.Now().UnixNano())
					}
					if ts == 0 {
						ts = ots
					}
					if ts < oldestAllowedTs {
						e.logger.Debug("skipping log", zap.Uint64("ts", ts), zap.Uint64("oldestAllowedTs", oldestAllowedTs))
						return nil
					}

					id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
					if err != nil {
						return fmt.Errorf("IdGenError:%w", err)
					}

					lBucketStart := tsBucket(int64(ts/1000000000), distributedLogsResourceV2Seconds)

					// fingerprint for resourceJson
					fp := resourcesSeen.getOrCreateFingerprint(
						int64(lBucketStart),
						resourceJson,
						func() string {
							return fingerprint.CalculateFingerprint(res.Attributes().AsRaw(), fingerprint.ResourceHierarchy())
						},
					)
					attrsMap := attributesToMap(record.Attributes(), false)

					if len(resourcesMap.StringData) > 100 {
						e.logger.Warn("resourcemap exceeded the limit of 100 keys")
					}
					// record size calculation
					attrBytes, _ := json.Marshal(record.Attributes().AsRaw())

					body, bodyJSON, promoted := e.processBody(record.Body())
					recordStream <- &Record{
						tsBucketStart:    uint64(lBucketStart),
						resourceFP:       fp,
						ts:               ts,
						ots:              ots,
						id:               id.String(),
						traceID:          utils.TraceIDToHexOrEmptyString(record.TraceID()),
						spanID:           utils.SpanIDToHexOrEmptyString(record.SpanID()),
						traceFlags:       uint32(record.Flags()),
						severityText:     record.SeverityText(),
						severityNum:      uint8(record.SeverityNumber()),
						body:             body,
						bodyJSON:         bodyJSON,
						bodyJSONPromoted: promoted,
						scopeName:        scopeName,
						scopeVersion:     scopeVersion,
						resourceMap:      resourcesMap,
						scopeMap:         scopeMap,
						attrsMap:         attrsMap,
						logFields:        attributeMap{StringData: map[string]string{"severity_text": record.SeverityText()}, NumberData: map[string]float64{"severity_number": float64(record.SeverityNumber())}},
						recordSize:       int64(len([]byte(record.Body().AsString())) + len(attrBytes) + len(resBytes)),
					}
					return nil
				})
			}
		}
	}

	// Close the record stream only after all producers finish
	// this is async so we block on group.Wait() and catch any error from the producer/consumer
	go func() {
		producersWG.Wait()
		closeRecordStream()
	}()

	err = group.Wait()
	if err != nil {
		return err
	}

	// Prepare and append resources fingerprints
	insertResourcesStmtV2, err = e.db.PrepareBatch(
		ctx,
		e.insertLogsResourceSQL,
		driver.WithReleaseConnection(),
	)
	if err != nil {
		return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
	}
	defer insertResourcesStmtV2.Close()

	err = resourcesSeen.rangeAll(func(bucketTs int64, resourceLables, fingerprint string) error {
		key := utils.MakeKeyForRFCache(bucketTs, fingerprint)
		if e.rfCache.Get(key) != nil {
			e.logger.Debug("resource fingerprint already present in cache, skipping", zap.String("key", key))
			return nil
		}
		if err := insertResourcesStmtV2.Append(
			resourceLables,
			fingerprint,
			bucketTs,
		); err != nil {
			return err
		}
		e.rfCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error appending resources to batch: %w", err)
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
		_ = stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, e.id.String())}, ExporterSigNozSentLogRecords.M(int64(v.Count)), ExporterSigNozSentLogRecordsBytes.M(int64(v.Size)))
	}

	return nil
}

func (e *clickhouseLogsExporter) processBody(body pcommon.Value) (string, string, string) {
	promoted := pcommon.NewValueMap()
	bodyJSON := pcommon.NewValueMap()
	if e.bodyJSONEnabled && body.Type() == pcommon.ValueTypeMap {
		// Work on a local mutable copy of the body to avoid mutating
		// the shared pdata across goroutines.
		mutableBody := pcommon.NewValueMap()
		body.CopyTo(mutableBody)

		// promoted paths extraction using cached set
		promotedSet := e.promotedPaths.Load().(map[string]struct{})

		// set values to promoted and bodyJSON
		promoted = buildPromotedAndPruneBody(mutableBody, promotedSet)
		bodyJSON = mutableBody

		if !e.bodyJSONOldBodyEnabled {
			// set body to empty string
			body = pcommon.NewValueEmpty()
		}
	}

	return getStringifiedBody(body), getStringifiedBody(bodyJSON), getStringifiedBody(promoted)
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
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, attrKey, tagType, utils.TagDataTypeString)
		if len(attrVal) > common.MaxAttributeValueLength {
			e.logger.Debug("attribute value length exceeds the limit", zap.String("key", attrKey))
			continue
		}

		key := utils.MakeKeyForAttributeKeys(attrKey, tagType, utils.TagDataTypeString)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
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
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, numKey, tagType, utils.TagDataTypeNumber)
		key := utils.MakeKeyForAttributeKeys(numKey, tagType, utils.TagDataTypeNumber)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}
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
		e.addAttrsToAttributeKeysStatement(attributeKeysStmt, resourceKeysStmt, boolKey, tagType, utils.TagDataTypeBool)

		key := utils.MakeKeyForAttributeKeys(boolKey, tagType, utils.TagDataTypeBool)
		if _, ok := shouldSkipKeys[key]; ok {
			e.logger.Debug("key has been skipped", zap.String("key", key))
			continue
		}

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

	// default settings for allowing ClickHouse to handle duplicate paths in JSON type.
	options.Settings["type_json_skip_duplicated_paths"] = 1

	// setting maxIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	maxIdleConnections := 1
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		maxIdleConnections = qc.NumConsumers + 1
	}
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

func renderInsertLogsSQLV2(includeBodyJSON bool) string {
	template := insertLogsSQLTemplateV2
	if includeBodyJSON {
		template = insertLogsSQLTemplateV2WithBodyJSON
	}
	return fmt.Sprintf(template, databaseName, distributedLogsTableV2)
}

func renderInsertLogsResourceSQL(_ *Config) string {
	return fmt.Sprintf(insertLogsResourceSQLTemplate, databaseName, distributedLogsResourceV2)
}
