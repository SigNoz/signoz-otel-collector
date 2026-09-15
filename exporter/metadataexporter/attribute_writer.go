package metadataexporter

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// attributeMetadataWriter writes resource+attribute fingerprint records to
// signoz_metadata.distributed_attributes_metadata.
type attributeMetadataWriter struct {
	conn   driver.Conn
	logger *zap.Logger

	bucket time.Duration

	shouldSkipFromDB      func(ctx context.Context, key, datasource string) bool
	filterAttrs           func(ctx context.Context, attrs map[string]any, datasource string) map[string]any
	filterIntrinsics      func(ctx context.Context, fields map[string]string, datasource string) map[string]string
	filterResourceAttrs   func(ctx context.Context, attrs map[string]any, datasource string) map[string]any
	writeToStatementBatch func(ctx context.Context, stmt driver.Batch, records []writeToStatementBatchRecord, ds pipeline.Signal) (int, error)
}

func newAttributeMetadataWriter(e *metadataExporter) *attributeMetadataWriter {
	return &attributeMetadataWriter{
		conn:                  e.conn,
		logger:                e.set.Logger,
		bucket:                e.cfg.MaxDistinctValues.Logs.Bucket,
		shouldSkipFromDB:      e.shouldSkipAttributeFromDB,
		filterAttrs:           e.filterAttrs,
		filterIntrinsics:      e.filterIntrinsics,
		filterResourceAttrs:   e.filterResourceAttrs,
		writeToStatementBatch: e.writeToStatementBatch,
	}
}

func (w *attributeMetadataWriter) Process(ctx context.Context, ld plog.Logs) error {
	stmt, err := w.conn.PrepareBatch(ctx, insertStmtQuery, driver.WithReleaseConnection())
	if err != nil {
		w.logger.Error("failed to prepare batch", zap.Error(err), zap.String("pipeline", pipeline.SignalLogs.String()))
		return nil
	}
	defer func() { _ = stmt.Close() }()

	totalLogRecords := 0
	records := make([]writeToStatementBatchRecord, 0)
	now := time.Now()

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resourceAttrs := make(map[string]any, rl.Resource().Attributes().Len())
		rl.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if w.shouldSkipFromDB(ctx, k, pipeline.SignalLogs.String()) {
				return true
			}
			resourceAttrs[k] = v.AsRaw()
			return true
		})
		flattenedResourceAttrs := w.filterResourceAttrs(ctx, flatten.FlattenJSON(resourceAttrs, ""), pipeline.SignalLogs.String())
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				totalLogRecords++
				logRecord := logs.At(k)
				logRecordAttrs := make(map[string]any, logRecord.Attributes().Len())

				logRecord.Attributes().Range(func(attrKey string, v pcommon.Value) bool {
					if attrKey == constants.OriginalBodyAttributeKey {
						return true
					}
					if w.shouldSkipFromDB(ctx, attrKey, pipeline.SignalLogs.String()) {
						return true
					}
					logRecordAttrs[attrKey] = v.AsRaw()
					return true
				})

				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				filteredLogRecordAttrs := w.filterAttrs(ctx, flattenedLogRecordAttrs, pipeline.SignalLogs.String())
				intrinsics := w.filterIntrinsics(ctx, logIntrinsics(logRecord), pipeline.SignalLogs.String())
				logRecordFingerprint := setFingerprint(filteredLogRecordAttrs, intrinsics)

				ts := logRecord.Timestamp()
				if ts == 0 {
					ts = logRecord.ObservedTimestamp()
				}
				roundedSixHrsUnixMilli := bucketStart(ts.AsTime(), now, w.bucket)

				records = append(records, writeToStatementBatchRecord{
					resourceFingerprint:    resourceFingerprint,
					fprint:                 logRecordFingerprint,
					rAttrs:                 flattenedResourceAttrs,
					attrs:                  filteredLogRecordAttrs,
					intrinsics:             intrinsics,
					roundedSixHrsUnixMilli: roundedSixHrsUnixMilli,
				})
			}
		}
	}

	written, err := w.writeToStatementBatch(ctx, stmt, records, pipeline.SignalLogs)
	if err != nil {
		w.logger.Error("failed to send stmt", zap.Error(err), zap.String("pipeline", pipeline.SignalLogs.String()))
	}
	skipped := totalLogRecords - written
	w.logger.Debug("pushed logs attributes", zap.Int("total_log_records", totalLogRecords), zap.Int("skipped_log_records", skipped))
	return nil
}
