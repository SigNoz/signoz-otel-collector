package signozclickhouseauditexporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

type auditExporter struct {
	logger    *zap.Logger
	cfg       *Config
	conn      driver.Conn
	telemetry *metadata.TelemetryBuilder
	tableAttr map[string]metric.RecordOption
	rfCache   *ttlcache.Cache[string, struct{}]
}

func newExporter(set exporter.Settings, cfg *Config) (*auditExporter, error) {
	telemetry, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	tableAttr := map[string]metric.RecordOption{}
	for _, table := range []string{distributedLogsTable, distributedLogsResourceTable} {
		tableAttr[table] = metric.WithAttributeSet(attribute.NewSet(attribute.String("table", table)))
	}

	return &auditExporter{
		logger:    set.Logger,
		cfg:       cfg,
		telemetry: telemetry,
		tableAttr: tableAttr,
		rfCache: ttlcache.New(
			ttlcache.WithTTL[string, struct{}](bucketSeconds*time.Second),
			ttlcache.WithCapacity[string, struct{}](100000),
			ttlcache.WithDisableTouchOnHit[string, struct{}](),
		),
	}, nil
}

func (e *auditExporter) start(ctx context.Context, _ component.Host) error {
	options, err := e.cfg.clickHouseOptions()
	if err != nil {
		return fmt.Errorf("failed to build clickhouse options: %w", err)
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return errors.Join(fmt.Errorf("failed to ping clickhouse: %w", err), conn.Close())
	}

	e.conn = conn
	go e.rfCache.Start()

	return nil
}

func (e *auditExporter) shutdown(_ context.Context) error {
	e.rfCache.Stop()
	e.telemetry.Shutdown()

	if e.conn != nil {
		return e.conn.Close()
	}

	return nil
}

func (e *auditExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	now := time.Now()
	b, err := e.buildBatch(ld, now)
	if err != nil {
		return err
	}

	if len(b.resources) > 0 {
		err := e.write(ctx, distributedLogsResourceTable, insertLogsResourceSQL, func(batch driver.Batch) error {
			for _, r := range b.resources {
				if err := batch.Append(r.labels, r.fingerprint, r.seenAtTSBucketStart); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if len(b.logs) > 0 {
		err := e.write(ctx, distributedLogsTable, insertLogsSQL, func(batch driver.Batch) error {
			for _, r := range b.logs {
				if err := batch.Append(r.tsBucketStart, r.resourceFingerprint, r.timestamp, r.observedTimestamp, r.id, r.traceID, r.spanID, r.traceFlags, r.severityText, r.severityNumber, r.body, r.scope, r.attributes, r.resource, r.eventName); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	for _, r := range b.resources {
		e.rfCache.Set(utils.MakeKeyForRFCache(r.seenAtTSBucketStart, r.fingerprint), struct{}{}, ttlcache.DefaultTTL)
	}

	e.logger.Debug("inserted audit logs",
		zap.Int("logs", len(b.logs)),
		zap.Int("resources", len(b.resources)),
		zap.Duration("duration", time.Since(now)),
	)

	return nil
}

func (e *auditExporter) write(ctx context.Context, table string, query string, appendRows func(driver.Batch) error) error {
	start := time.Now()

	batch, err := e.conn.PrepareBatch(ctx, query, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare batch for %s: %w", table, err)
	}
	defer func() { _ = batch.Close() }()

	if err := appendRows(batch); err != nil {
		return fmt.Errorf("failed to append to %s: %w", table, err)
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch to %s: %w", table, err)
	}

	e.telemetry.SignozclickhouseauditWriteDuration.Record(ctx, time.Since(start).Seconds(), e.tableAttr[table])

	return nil
}
