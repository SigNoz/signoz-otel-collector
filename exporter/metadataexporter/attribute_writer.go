package metadataexporter

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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

	shouldSkipFromDB      func(ctx context.Context, key, datasource string) bool
	filterAttrs           func(ctx context.Context, attrs map[string]any, datasource string) map[string]any
	writeToStatementBatch func(ctx context.Context, stmt driver.Batch, records []writeToStatementBatchRecord, ds pipeline.Signal) (int, error)
}

func newAttributeMetadataWriter(e *metadataExporter) *attributeMetadataWriter {
	return &attributeMetadataWriter{
		conn:                  e.conn,
		logger:                e.set.Logger,
		shouldSkipFromDB:      e.shouldSkipAttributeFromDB,
		filterAttrs:           e.filterAttrs,
		writeToStatementBatch: e.writeToStatementBatch,
	}
}

func (w *attributeMetadataWriter) Process(ctx context.Context, ld plog.Logs) error {
	stmt, err := w.conn.PrepareBatch(ctx, insertStmtQuery)
	if err != nil {
		w.logger.Error("failed to prepare batch", zap.Error(err), zap.String("pipeline", pipeline.SignalLogs.String()))
		return nil
	}
	defer stmt.Close()

	totalLogRecords := 0
	records := make([]writeToStatementBatchRecord, 0)

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
		flattenedResourceAttrs := flatten.FlattenJSON(resourceAttrs, "")
		resourceFingerprint := fingerprint.FingerprintHash(flattenedResourceAttrs)

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				totalLogRecords++
				logRecord := logs.At(k)
				logRecordAttrs := make(map[string]any, logRecord.Attributes().Len())

				logRecord.Attributes().Range(func(attrKey string, v pcommon.Value) bool {
					if w.shouldSkipFromDB(ctx, attrKey, pipeline.SignalLogs.String()) {
						return true
					}
					logRecordAttrs[attrKey] = v.AsRaw()
					return true
				})

				flattenedLogRecordAttrs := flatten.FlattenJSON(logRecordAttrs, "")
				filteredLogRecordAttrs := w.filterAttrs(ctx, flattenedLogRecordAttrs, pipeline.SignalLogs.String())
				logRecordFingerprint := fingerprint.FingerprintHash(filteredLogRecordAttrs)

				unixMilli := logRecord.Timestamp().AsTime().UnixMilli()
				roundedSixHrsUnixMilli := (unixMilli / sixHoursInMs) * sixHoursInMs

				records = append(records, writeToStatementBatchRecord{
					resourceFingerprint:    resourceFingerprint,
					fprint:                 logRecordFingerprint,
					rAttrs:                 flattenedResourceAttrs,
					attrs:                  filteredLogRecordAttrs,
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
