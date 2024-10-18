package clickhousetracesexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func (w *SpanWriter) writeIndexBatchV2(ctx context.Context, batchSpans []*SpanV2) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, "distributed_signoz_index_v3"), driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("Could not prepare batch for index table: ", zap.Error(err))
		return err
	}

	for _, span := range batchSpans {
		err = statement.Append(
			span.TsBucketStart,
			span.FingerPrint,
			time.Unix(0, int64(span.StartTimeUnixNano)),
			span.Id,
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

			span.ServiceName,

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

			span.HttpRoute,
			span.MsgSystem,
			span.MsgOperation,
			span.DBSystem,
			span.RPCSystem,
			span.RPCService,
			span.RPCMethod,
			span.PeerService,

			span.References,
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
		tag.Upsert(tableKey, w.indexTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return err
}

// WriteBatchOfSpans writes the encoded batch of spans
func (w *SpanWriter) WriteBatchOfSpansV2(ctx context.Context, batch []*SpanV2) error {
	// inserts to the signoz_index_3 table
	if w.indexTable != "" {
		if err := w.writeIndexBatchV2(ctx, batch); err != nil {
			w.logger.Error("Could not write a batch of spans to index table: ", zap.Error(err))
			return err
		}
	}

	// inserts to the signoz_error_index_v2 table
	// TODO(nitya): move them here once we remove writer.go
	// if w.errorTable != "" {
	// 	if err := w.writeErrorBatchV2(ctx, batch); err != nil {
	// 		w.logger.Error("Could not write a batch of spans to error table: ", zap.Error(err))
	// 		return err
	// 	}
	// }
	// if w.attributeTable != "" && w.attributeKeyTable != "" {
	// 	if err := w.writeTagBatchv2(ctx, batch); err != nil {
	// 		w.logger.Error("Could not write a batch of spans to tag/tagKey tables: ", zap.Error(err))
	// 		return err
	// 	}
	// }
	return nil
}

func (w *SpanWriter) WriteResourcesV2(ctx context.Context, resourcesSeen map[int64]map[string]string) error {
	var insertResourcesStmtV2 driver.Batch

	defer func() {
		if insertResourcesStmtV2 != nil {
			_ = insertResourcesStmtV2.Abort()
		}
	}()

	insertResourcesStmtV2, err := w.db.PrepareBatch(
		ctx,
		fmt.Sprintf("INSERT into %s.%s", w.traceDatabase, "distributed_traces_v3_resource"),
		driver.WithReleaseConnection(),
	)
	if err != nil {
		return fmt.Errorf("couldn't PrepareBatch for inserting resource fingerprints :%w", err)
	}

	for bucketTs, resources := range resourcesSeen {
		for resourceLabels, fingerprint := range resources {
			insertResourcesStmtV2.Append(
				resourceLabels,
				fingerprint,
				bucketTs,
			)
		}
	}
	err = insertResourcesStmtV2.Send()
	if err != nil {
		return fmt.Errorf("couldn't send resource fingerprints :%w", err)
	}
	return nil
}
