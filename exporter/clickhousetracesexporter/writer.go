// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"context"
	"fmt"
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
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
)

const (
	distributedTracesResourceV2Seconds = 1800
)

type shouldSkipKey struct {
	TagKey      string `ch:"tag_key"`
	TagType     string `ch:"tag_type"`
	TagDataType string `ch:"tag_data_type"`
	StringCount uint64 `ch:"string_count"`
	NumberCount uint64 `ch:"number_count"`
}

// SpanWriter for writing spans to ClickHouse
type SpanWriter struct {
	logger            *zap.Logger
	db                clickhouse.Conn
	traceDatabase     string
	errorTable        string
	attributeTableV2  string
	attributeKeyTable string
	exporterId        uuid.UUID
	durationHistogram metric.Float64Histogram

	indexTableV3    string
	resourceTableV3 string

	keysCache *ttlcache.Cache[string, struct{}]
	rfCache   *ttlcache.Cache[string, struct{}]

	shouldSkipKeyValue atomic.Value // stores map[string]shouldSkipKey

	maxDistinctValues         int
	fetchShouldSkipKeysTicker *time.Ticker
	done                      chan struct{}
}

// NewSpanWriter returns a SpanWriter for the database
func NewSpanWriter(options ...WriterOption) *SpanWriter {

	writer := &SpanWriter{
		traceDatabase:     defaultTraceDatabase,
		errorTable:        defaultErrorTable,
		attributeTableV2:  defaultAttributeTableV2,
		attributeKeyTable: defaultAttributeKeyTable,
		indexTableV3:      defaultIndexTableV3,
		resourceTableV3:   defaultResourceTableV3,
		done:              make(chan struct{}),
	}
	for _, option := range options {
		option(writer)
	}

	// Fetch keys immediately, then start the background ticker routine
	go func() {
		writer.doFetchShouldSkipKeys() // Immediate first fetch
		writer.fetchShouldSkipKeys()   // Start ticker routine
	}()

	return writer
}

// doFetchShouldSkipKeys contains the logic for fetching skip keys
func (e *SpanWriter) doFetchShouldSkipKeys() {
	query := fmt.Sprintf(`
		SELECT tag_key, tag_type, tag_data_type, countDistinct(string_value) as string_count, countDistinct(number_value) as number_count
		FROM %s.%s
		WHERE unix_milli >= (toUnixTimestamp(now() - toIntervalHour(6)) * 1000)
		GROUP BY tag_key, tag_type, tag_data_type
		HAVING string_count > %d OR number_count > %d
		SETTINGS max_threads = 2`, e.traceDatabase, e.attributeTableV2, e.maxDistinctValues, e.maxDistinctValues)

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

func (e *SpanWriter) fetchShouldSkipKeys() {
	for {
		select {
		case <-e.done:
			return
		case <-e.fetchShouldSkipKeysTicker.C:
			e.doFetchShouldSkipKeys()
		}
	}
}

func (w *SpanWriter) writeIndexBatchV3(ctx context.Context, batchSpans []*SpanV3) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf(insertTraceSQLTemplateV2, w.traceDatabase, w.indexTableV3), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for index table: %w", err)
	}
	defer statement.Close()

	for _, span := range batchSpans {

		if len(span.ResourcesString) > 100 {
			w.logger.Warn("resourcemap exceeded the limit of 100 keys")
		}

		err = statement.Append(
			span.TsBucketStart,
			span.FingerPrint,
			time.Unix(0, int64(span.StartTimeUnixNano)),
			span.TraceId,
			span.SpanId,
			span.TraceState,
			span.ParentSpanId,
			span.Flags,
			span.Name,
			span.Kind,
			span.SpanKind,
			span.DurationNano,
			span.StatusCode,
			span.StatusMessage,
			span.StatusCodeString,
			span.AttributeString,
			span.AttributesNumber,
			span.AttributesBool,
			span.ResourcesString,
			span.ResourcesString,
			span.Events,
			span.References,

			// composite attributes
			span.ResponseStatusCode,
			span.ExternalHttpUrl,
			span.HttpUrl,
			span.ExternalHttpMethod,
			span.HttpMethod,
			span.HttpHost,
			span.DBName,
			span.DBOperation,
			span.HasError,
			span.IsRemote,
		)
		if err != nil {
			return fmt.Errorf("could not append span to batch: %w", err)
		}
	}

	start := time.Now()

	err = statement.Send()

	w.durationHistogram.Record(
		ctx,
		float64(time.Since(start).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.indexTableV3),
		),
	)
	return err
}

func (w *SpanWriter) writeErrorBatchV3(ctx context.Context, batchSpans []*SpanV3) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.errorTable), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for error table: %w", err)
	}
	defer statement.Close()

	for _, span := range batchSpans {
		for _, errorEvent := range span.ErrorEvents {
			if errorEvent.Event.Name == "" {
				continue
			}
			err = statement.Append(
				time.Unix(0, int64(errorEvent.Event.TimeUnixNano)),
				errorEvent.ErrorID,
				errorEvent.ErrorGroupID,
				span.TraceId,
				span.SpanId,
				span.ServiceName,
				errorEvent.Event.AttributeMap[string(semconv.ExceptionTypeKey)],
				errorEvent.Event.AttributeMap[string(semconv.ExceptionMessageKey)],
				errorEvent.Event.AttributeMap[string(semconv.ExceptionStacktraceKey)],
				stringToBool(errorEvent.Event.AttributeMap[string(semconv.ExceptionEscapedKey)]),
				span.ResourcesString,
			)
			if err != nil {
				return fmt.Errorf("could not append span to batch: %w", err)
			}
		}
	}

	start := time.Now()

	err = statement.Send()

	w.durationHistogram.Record(
		ctx,
		float64(time.Since(start).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.errorTable),
		),
	)
	return err
}

func (w *SpanWriter) writeTagBatchV3(ctx context.Context, batchSpans []*SpanV3) error {
	var tagKeyStatement driver.Batch
	var tagStatementV2 driver.Batch
	var err error
	var shouldSkipKeys map[string]shouldSkipKey

	if keys := w.shouldSkipKeyValue.Load(); keys != nil {
		shouldSkipKeys = keys.(map[string]shouldSkipKey)
	}

	tagKeyStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeKeyTable), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for span attributes key table due to error: %w", err)
	}
	defer tagKeyStatement.Close()

	tagStatementV2, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeTableV2), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for span attributes table v2 due to error: %w", err)
	}
	defer tagStatementV2.Close()

	// create map of span attributes of key, tagType, dataType and isColumn to avoid duplicates in batch
	mapOfSpanAttributeKeys := make(map[string]struct{})

	// create map of span attributes of key, tagType, dataType, isColumn and value to avoid duplicates in batch
	mapOfSpanAttributeValues := make(map[string]struct{})

	mapOfSpanFields := make(map[string]struct{})

	for _, span := range batchSpans {
		unixMilli := (int64(span.StartTimeUnixNano/1e6) / 3600000) * 3600000

		for _, spanAttribute := range span.SpanAttributes {

			// form a map key of span attribute key, tagType, dataType, isColumn and value
			mapOfSpanAttributeValueKey := spanAttribute.Key + spanAttribute.TagType + spanAttribute.DataType + spanAttribute.StringValue + strconv.FormatFloat(spanAttribute.NumberValue, 'f', -1, 64)

			// check if mapOfSpanAttributeValueKey already exists in map
			_, ok := mapOfSpanAttributeValues[mapOfSpanAttributeValueKey]
			if ok {
				continue
			}
			// add mapOfSpanAttributeValueKey to map
			mapOfSpanAttributeValues[mapOfSpanAttributeValueKey] = struct{}{}

			// form a map key of span attribute key, tagType, dataType and isColumn
			mapOfSpanAttributeKey := spanAttribute.Key + spanAttribute.TagType + spanAttribute.DataType + strconv.FormatBool(spanAttribute.IsColumn)

			// check if mapOfSpanAttributeKey already exists in map
			_, ok = mapOfSpanAttributeKeys[mapOfSpanAttributeKey]
			if !ok {
				if w.keysCache.Get(mapOfSpanAttributeKey) == nil {
					err = tagKeyStatement.Append(
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						spanAttribute.IsColumn,
					)
					if err != nil {
						return fmt.Errorf("could not append span to tagKey Statement to batch due to error: %w", err)
					}
					w.keysCache.Set(mapOfSpanAttributeKey, struct{}{}, ttlcache.DefaultTTL)
				} else {
					w.logger.Debug("attribute key already present in cache, skipping", zap.String("key", mapOfSpanAttributeKey))
				}
			}
			// add mapOfSpanAttributeKey to map
			mapOfSpanAttributeKeys[mapOfSpanAttributeKey] = struct{}{}
			v2Key := utils.MakeKeyForAttributeKeys(spanAttribute.Key, utils.TagType(spanAttribute.TagType), utils.TagDataType(spanAttribute.DataType))

			if len(spanAttribute.StringValue) > common.MaxAttributeValueLength {
				w.logger.Debug("attribute value length exceeds the limit", zap.String("key", spanAttribute.Key))
				continue
			}

			if spanAttribute.DataType == "string" {

				if _, ok := shouldSkipKeys[v2Key]; !ok {
					// TODO: handle error
					_ = tagStatementV2.Append(
						unixMilli,
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						spanAttribute.StringValue,
						nil,
					)
				}

			} else if spanAttribute.DataType == "float64" {
				if _, ok = shouldSkipKeys[v2Key]; !ok {
					// TODO: handle error
					_ = tagStatementV2.Append(
						unixMilli,
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						nil,
						spanAttribute.NumberValue,
					)
				}
			} else if spanAttribute.DataType == "bool" {
				if _, ok = shouldSkipKeys[v2Key]; !ok {
					// TODO: handle erroe
					_ = tagStatementV2.Append(
						unixMilli,
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						nil,
						nil,
					)
				}
			}
			if err != nil {
				return fmt.Errorf("could not append span to tag Statement batch due to error: %w", err)
			}
		}

		// span fields
		// name, kind, kind_string, status_code_string, status_code
		if _, ok := mapOfSpanFields[span.Name]; !ok {
			mapOfSpanFields[span.Name] = struct{}{}
			// TODO: handle error
			_ = tagStatementV2.Append(unixMilli, "name", utils.TagTypeSpanField, utils.TagDataTypeString, span.Name, nil)
		}
		if _, ok := mapOfSpanFields[span.SpanKind]; !ok {
			mapOfSpanFields[span.SpanKind] = struct{}{}
			// TODO: handle error
			_ = tagStatementV2.Append(unixMilli, "kind_string", utils.TagTypeSpanField, utils.TagDataTypeString, span.SpanKind, nil)
			_ = tagStatementV2.Append(unixMilli, "kind", utils.TagTypeSpanField, utils.TagDataTypeNumber, nil, float64(span.Kind))
		}
		if _, ok := mapOfSpanFields[span.StatusCodeString]; !ok {
			mapOfSpanFields[span.StatusCodeString] = struct{}{}
			// TODO: handle error
			_ = tagStatementV2.Append(unixMilli, "status_code_string", utils.TagTypeSpanField, utils.TagDataTypeString, span.StatusCodeString, nil)
			_ = tagStatementV2.Append(unixMilli, "status_code", utils.TagTypeSpanField, utils.TagDataTypeNumber, nil, float64(span.StatusCode))
		}
	}

	tagStart := time.Now()
	err = tagStatementV2.Send()
	w.durationHistogram.Record(
		ctx,
		float64(time.Since(tagStart).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.attributeTableV2),
		),
	)
	if err != nil {
		return fmt.Errorf("could not write to span attributes table due to error: %w", err)
	}

	tagKeyStart := time.Now()
	err = tagKeyStatement.Send()
	w.durationHistogram.Record(
		ctx,
		float64(time.Since(tagKeyStart).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.attributeKeyTable),
		),
	)
	if err != nil {
		return fmt.Errorf("could not write to span attributes key table due to error: %w", err)
	}

	return err
}

// WriteBatchOfSpans writes the encoded batch of spans
func (w *SpanWriter) WriteBatchOfSpansV3(ctx context.Context, batch []*SpanV3, metrics map[string]usage.Metric) error {
	var wg sync.WaitGroup
	var chErr = make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.writeIndexBatchV3(ctx, batch)
		if err != nil {
			w.logger.Error("Could not write a batch of spans to index table: ", zap.Error(err))
			chErr <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.writeErrorBatchV3(ctx, batch)
		if err != nil {
			w.logger.Error("Could not write a batch of spans to error table: ", zap.Error(err))
			chErr <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := w.writeTagBatchV3(ctx, batch)
		if err != nil {
			w.logger.Error("Could not write a batch of spans to tag/tagKey tables: ", zap.Error(err))
			// Not returning the error as we don't to block the exporter
			// chErr <- err
		}
	}()

	wg.Wait()

	for k, v := range metrics {
		err := stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, w.exporterId.String())}, ExporterSigNozSentSpans.M(int64(v.Count)), ExporterSigNozSentSpansBytes.M(int64(v.Size)))
		if err != nil {
			w.logger.Error("WriteBatchOfSpansV3 usage metric error", zap.Error(err))
		}
	}

	close(chErr)
	for i := 0; i < 2; i++ {
		if r := <-chErr; r != nil {
			return fmt.Errorf("TracesWriteBatchOfSpansV3:%w", r)
		}
	}

	return nil
}

func (w *SpanWriter) WriteResourcesV3(ctx context.Context, resourcesSeen map[int64]map[string]string) error {
	var insertResourcesStmtV3 driver.Batch

	defer func() {
		if insertResourcesStmtV3 != nil {
			_ = insertResourcesStmtV3.Abort()
		}
	}()

	insertResourcesStmtV3, err := w.db.PrepareBatch(
		ctx,
		fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.resourceTableV3),
		driver.WithReleaseConnection(),
	)
	if err != nil {
		return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
	}
	defer insertResourcesStmtV3.Close()

	for bucketTs, resources := range resourcesSeen {
		for resourceLabels, fingerprint := range resources {
			key := utils.MakeKeyForRFCache(bucketTs, fingerprint)
			if w.rfCache.Get(key) != nil {
				w.logger.Debug("resource fingerprint already present in cache, skipping", zap.String("key", key))
				continue
			}
			// TODO: handle error
			_ = insertResourcesStmtV3.Append(
				resourceLabels,
				fingerprint,
				bucketTs,
			)
			w.rfCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
		}
	}
	start := time.Now()

	err = insertResourcesStmtV3.Send()
	if err != nil {
		return fmt.Errorf("couldn't send resource fingerprints :%w", err)
	}

	w.durationHistogram.Record(
		ctx,
		float64(time.Since(start).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.resourceTableV3),
		),
	)
	return nil
}

func stringToBool(s string) bool {
	return strings.ToLower(s) == "true"
}

func (w *SpanWriter) Stop() {
	if w.fetchShouldSkipKeysTicker != nil {
		w.fetchShouldSkipKeysTicker.Stop()
	}
	if w.keysCache != nil {
		w.keysCache.Stop()
	}
	if w.rfCache != nil {
		w.rfCache.Stop()
	}
	close(w.done)
}

// Close closes the writer
func (w *SpanWriter) Close() error {
	w.Stop()
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}
