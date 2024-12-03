package clickhousetracesexporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.uber.org/zap"
)

func makeKeyForRFCache(bucketTs int64, fingerprint string) string {
	var v strings.Builder
	v.WriteString(strconv.Itoa(int(bucketTs)))
	v.WriteString(":")
	v.WriteString(fingerprint)
	return v.String()
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
		w.logger.Error("Could not prepare batch for index table: ", zap.Error(err))
		return err
	}
	for _, span := range batchSpans {
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
			w.logger.Error("Could not append span to batch: ", zap.Any("span", span), zap.Error(err))
			return err
		}
	}

	start := time.Now()

	err = statement.Send()

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.indexTableV3),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
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
		w.logger.Error("Could not prepare batch for error table: ", zap.Error(err))
		return err
	}

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
				errorEvent.Event.AttributeMap[semconv.AttributeExceptionType],
				errorEvent.Event.AttributeMap[semconv.AttributeExceptionMessage],
				errorEvent.Event.AttributeMap[semconv.AttributeExceptionStacktrace],
				stringToBool(errorEvent.Event.AttributeMap[semconv.AttributeExceptionEscaped]),
				span.ResourcesString,
			)
			if err != nil {
				w.logger.Error("Could not append span to batch: ", zap.Any("span", span), zap.Error(err))
				return err
			}
		}
	}

	start := time.Now()

	err = statement.Send()

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.errorTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return err
}

func (w *SpanWriter) writeTagBatchV3(ctx context.Context, batchSpans []*SpanV3) error {
	var tagKeyStatement driver.Batch
	var tagStatement driver.Batch
	var tagStatementV2 driver.Batch
	var err error

	fetchKeys := make(map[string]shouldSkipKey)
	w.fetchKeysMutex.RLock()
	for k, v := range w.shouldSkipKey {
		fetchKeys[k] = shouldSkipKey{
			TagKey:      v.TagKey,
			TagType:     v.TagType,
			TagDataType: v.TagDataType,
			StringCount: v.StringCount,
			NumberCount: v.NumberCount,
		}
	}
	w.fetchKeysMutex.RUnlock()

	defer func() {
		if tagKeyStatement != nil {
			_ = tagKeyStatement.Abort()
		}
		if tagStatement != nil {
			_ = tagStatement.Abort()
		}
		if tagStatementV2 != nil {
			_ = tagStatementV2.Abort()
		}
	}()
	tagStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeTable), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for span attributes table due to error: ", zap.Error(err))
		return err
	}
	tagStatementV2, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeTableV2), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for span attributes table v2 due to error: ", zap.Error(err))
		return err
	}
	tagKeyStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeKeyTable), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for span attributes key table due to error: ", zap.Error(err))
		return err
	}
	// create map of span attributes of key, tagType, dataType and isColumn to avoid duplicates in batch
	mapOfSpanAttributeKeys := make(map[string]struct{})

	// create map of span attributes of key, tagType, dataType, isColumn and value to avoid duplicates in batch
	mapOfSpanAttributeValues := make(map[string]struct{})

	for _, span := range batchSpans {
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
						w.logger.Error("Could not append span to tagKey Statement to batch due to error: ", zap.Error(err), zap.Any("span", span))
						return err
					}
					w.keysCache.Set(mapOfSpanAttributeKey, struct{}{}, ttlcache.DefaultTTL)
				}
			}
			// add mapOfSpanAttributeKey to map
			mapOfSpanAttributeKeys[mapOfSpanAttributeKey] = struct{}{}
			key := makeKey(spanAttribute.Key, spanAttribute.TagType, spanAttribute.DataType)

			if spanAttribute.DataType == "string" {
				// err = tagStatement.Append(
				// 	time.Unix(0, int64(span.StartTimeUnixNano)),
				// 	spanAttribute.Key,
				// 	spanAttribute.TagType,
				// 	spanAttribute.DataType,
				// 	spanAttribute.StringValue,
				// 	nil,
				// 	spanAttribute.IsColumn,
				// )
				_, ok = fetchKeys[key]
				if !ok {
					err = tagStatementV2.Append(
						time.Unix(0, int64(span.StartTimeUnixNano)),
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						spanAttribute.StringValue,
						nil,
					)
				}
			} else if spanAttribute.DataType == "float64" {
				// err = tagStatement.Append(
				// 	time.Unix(0, int64(span.StartTimeUnixNano)),
				// 	spanAttribute.Key,
				// 	spanAttribute.TagType,
				// 	spanAttribute.DataType,
				// 	nil,
				// 	spanAttribute.NumberValue,
				// 	spanAttribute.IsColumn,
				// )
				_, ok = fetchKeys[key]
				if !ok {
					err = tagStatementV2.Append(
						time.Unix(0, int64(span.StartTimeUnixNano)),
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						nil,
						spanAttribute.NumberValue,
					)
				}
			} else if spanAttribute.DataType == "bool" {
				// err = tagStatement.Append(
				// 	time.Unix(0, int64(span.StartTimeUnixNano)),
				// 	spanAttribute.Key,
				// 	spanAttribute.TagType,
				// 	spanAttribute.DataType,
				// 	nil,
				// 	nil,
				// 	spanAttribute.IsColumn,
				// )
				_, ok = fetchKeys[key]
				if !ok {
					err = tagStatementV2.Append(
						time.Unix(0, int64(span.StartTimeUnixNano)),
						spanAttribute.Key,
						spanAttribute.TagType,
						spanAttribute.DataType,
						nil,
						nil,
					)
				}
			}
			if err != nil {
				w.logger.Error("Could not append span to tag Statement batch due to error: ", zap.Error(err), zap.Any("span", span))
				return err
			}
		}
	}

	tagStart := time.Now()
	// err1 := tagStatement.Send()
	err2 := tagStatementV2.Send()
	stats.RecordWithTags(ctx,
		[]tag.Mutator{
			tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
			tag.Upsert(tableKey, w.attributeTable),
		},
		writeLatencyMillis.M(int64(time.Since(tagStart).Milliseconds())),
	)
	if err2 != nil {
		w.logger.Error("Could not write to span attributes table due to error: ", zap.Error(err2))
		return fmt.Errorf("Could not write to span attributes table due to error: %w", err2)
	}

	tagKeyStart := time.Now()
	err = tagKeyStatement.Send()
	stats.RecordWithTags(ctx,
		[]tag.Mutator{
			tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
			tag.Upsert(tableKey, w.attributeKeyTable),
		},
		writeLatencyMillis.M(int64(time.Since(tagKeyStart).Milliseconds())),
	)
	if err != nil {
		w.logger.Error("Could not write to span attributes key table due to error: ", zap.Error(err))
		return err
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

	if w.useNewSchema {
		for k, v := range metrics {
			stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, w.exporterId.String())}, ExporterSigNozSentSpans.M(int64(v.Count)), ExporterSigNozSentSpansBytes.M(int64(v.Size)))
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
		fmt.Sprintf("INSERT into %s.%s", w.traceDatabase, w.resourceTableV3),
		driver.WithReleaseConnection(),
	)
	if err != nil {
		return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
	}

	for bucketTs, resources := range resourcesSeen {
		for resourceLabels, fingerprint := range resources {
			key := makeKeyForRFCache(bucketTs, fingerprint)
			if w.rfCache.Get(key) != nil {
				continue
			}
			insertResourcesStmtV3.Append(
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

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.resourceTableV3),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return nil
}

const (
	insertTraceSQLTemplateV2 = `INSERT INTO %s.%s (
							ts_bucket_start,
							resource_fingerprint,
							timestamp,
							trace_id,
							span_id,
							trace_state,
							parent_span_id,
							flags,
							name,
							kind,
							kind_string,
							duration_nano,
							status_code,
							status_message,
							status_code_string,
							attributes_string,
							attributes_number,
							attributes_bool,
							resources_string,
							events,
							links,
							response_status_code,
							external_http_url,
							http_url,
							external_http_method,
							http_method,
							http_host,
							db_name,
							db_operation,
							has_error,
							is_remote
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
