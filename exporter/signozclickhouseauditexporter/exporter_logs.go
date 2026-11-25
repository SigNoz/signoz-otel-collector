package signozclickhouseauditexporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/sqltemplates"
	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
)

const (
	bucketSeconds = 1800

	database                     = "signoz_audit"
	distributedLogsTable         = "distributed_logs"
	distributedLogsResourceTable = "distributed_logs_resource"
)

type auditLogsExporter struct {
	db                driver.Conn
	insertSQL         string
	insertResourceSQL string
	resourceSet       *timebucketedset.Set
	telemetry         *metadata.TelemetryBuilder
	tableAttr         map[string]metric.RecordOption

	logger *zap.Logger
	cfg    *Config
}

func newAuditLogsExporter(set exporter.Settings, cfg *Config) (*auditLogsExporter, error) {
	telemetry, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	tableAttr := map[string]metric.RecordOption{}
	for _, table := range []string{distributedLogsTable, distributedLogsResourceTable} {
		tableAttr[table] = metric.WithAttributeSet(attribute.NewSet(attribute.String("table", table)))
	}

	resourceSet, err := timebucketedset.New(
		bucketSeconds*time.Second,
		timebucketedset.Config{MaxBuckets: 2, MaxBucketSize: 32 << 20},
		set.TelemetrySettings,
		attribute.String("exporter", set.ID.String()),
	)
	if err != nil {
		telemetry.Shutdown()
		return nil, fmt.Errorf("failed to create resource set: %w", err)
	}

	e := &auditLogsExporter{
		resourceSet: resourceSet,
		telemetry:   telemetry,
		tableAttr:   tableAttr,
		logger:      set.Logger,
		cfg:         cfg,
	}

	if err := e.renderInsertLogsSQL(); err != nil {
		resourceSet.Shutdown()
		telemetry.Shutdown()
		return nil, fmt.Errorf("render logs insert sql: %w", err)
	}

	return e, nil
}

func (e *auditLogsExporter) start(_ context.Context, _ component.Host) error {
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = clickhouse.Open(opt)
	if err != nil {
		return err
	}

	return nil
}

func (e *auditLogsExporter) shutdown(_ context.Context) error {
	e.resourceSet.Shutdown()
	e.telemetry.Shutdown()

	if e.db != nil {
		return e.db.Close()
	}

	return nil
}

func (e *auditLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	processStart := time.Now()
	b, err := e.buildBatch(ld, processStart)
	if err != nil {
		return err
	}

	var resourceBatch, logsBatch driver.Batch

	if len(b.resources) > 0 {
		resourceBatch, err = e.db.PrepareBatch(ctx, e.insertResourceSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		defer func(batch driver.Batch) {
			if closeErr := batch.Close(); closeErr != nil {
				e.logger.Warn("failed to close logs resource batch", zap.Error(closeErr))
			}
		}(resourceBatch)

		for _, r := range b.resources {
			if err := resourceBatch.Append(r.labels, r.fingerprint, r.seenAtTSBucketStart); err != nil {
				return fmt.Errorf("failed to append log resource row: %w", err)
			}
		}
	}

	if len(b.logs) > 0 {
		logsBatch, err = e.db.PrepareBatch(ctx, e.insertSQL, driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		defer func(batch driver.Batch) {
			if closeErr := batch.Close(); closeErr != nil {
				e.logger.Warn("failed to close logs batch", zap.Error(closeErr))
			}
		}(logsBatch)

		for _, r := range b.logs {
			if err := logsBatch.Append(
				r.tsBucketStart,
				r.resourceFingerprint,
				r.timestamp,
				r.observedTimestamp,
				r.id,
				r.traceID,
				r.spanID,
				r.traceFlags,
				r.severityText,
				r.severityNumber,
				r.body,
				r.scope,
				r.attributes,
				r.resource,
				r.eventName,
			); err != nil {
				return fmt.Errorf("failed to append log row: %w", err)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()

	errCh := make(chan error, 2)
	go func() { errCh <- e.flush(ctx, resourceBatch, distributedLogsResourceTable) }()
	go func() { errCh <- e.flush(ctx, logsBatch, distributedLogsTable) }()

	if err := errors.Join(<-errCh, <-errCh); err != nil {
		return fmt.Errorf("logs insert failed: %w", err)
	}

	e.resourceSet.Apply(func(yield func([]byte, int64) bool) {
		for _, r := range b.resources {
			if !yield([]byte(r.fingerprint), r.seenAtTSBucketStart*1000) {
				return
			}
		}
	})

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert logs",
		zap.Int("records", len(b.logs)),
		zap.Int("resources", len(b.resources)),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (e *auditLogsExporter) renderInsertLogsSQL() error {
	var buf bytes.Buffer
	if err := sqltemplates.LogsInsertTmpl.Execute(&buf, sqltemplates.InsertData{Database: database, TableName: distributedLogsTable}); err != nil {
		return fmt.Errorf("execute logs insert template: %w", err)
	}
	e.insertSQL = buf.String()

	buf.Reset()
	if err := sqltemplates.LogsResourceInsertTmpl.Execute(&buf, sqltemplates.InsertData{Database: database, TableName: distributedLogsResourceTable}); err != nil {
		return fmt.Errorf("execute logs resource insert template: %w", err)
	}
	e.insertResourceSQL = buf.String()

	return nil
}

func (e *auditLogsExporter) flush(ctx context.Context, batch driver.Batch, table string) error {
	if batch == nil {
		return nil
	}

	start := time.Now()
	err := batch.Send()
	e.telemetry.SignozclickhouseauditWriteDuration.Record(ctx, time.Since(start).Seconds(), e.tableAttr[table])
	return err
}
