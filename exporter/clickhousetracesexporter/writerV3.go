package clickhousetracesexporter

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

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
			span.ServerAddress,
			span.UrlPath,
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

	defer func() {
		if tagKeyStatement != nil {
			_ = tagKeyStatement.Abort()
		}
		if tagStatementV2 != nil {
			_ = tagStatementV2.Abort()
		}
	}()
	tagKeyStatement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeKeyTable), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for span attributes key table due to error: %w", err)
	}
	tagStatementV2, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.attributeTableV2), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for span attributes table v2 due to error: %w", err)
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
			unixMilli := (int64(span.StartTimeUnixNano/1e6) / 3600000) * 3600000

			if len(spanAttribute.StringValue) > common.MaxAttributeValueLength {
				w.logger.Debug("attribute value length exceeds the limit", zap.String("key", spanAttribute.Key))
				continue
			}

			if spanAttribute.DataType == "string" {

				if _, ok := shouldSkipKeys[v2Key]; !ok {
					err = tagStatementV2.Append(
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
					err = tagStatementV2.Append(
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
					err = tagStatementV2.Append(
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
	}

	tagStart := time.Now()
	err = tagStatementV2.Send()
	w.durationHistogram.Record(
		ctx,
		float64(time.Since(tagStart).Milliseconds()),
		metric.WithAttributes(
			attribute.String("exporter", pipeline.SignalTraces.String()),
			attribute.String("table", w.attributeTable),
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
			key := utils.MakeKeyForRFCache(bucketTs, fingerprint)
			if w.rfCache.Get(key) != nil {
				w.logger.Debug("resource fingerprint already present in cache, skipping", zap.String("key", key))
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
