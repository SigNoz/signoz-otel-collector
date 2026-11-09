package signozclickhouseauditexporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/goccy/go-json"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
)

const bucketSeconds = 1800

type logRow struct {
	tsBucketStart       uint64
	resourceFingerprint string
	timestamp           uint64
	observedTimestamp   uint64
	id                  string
	traceID             string
	spanID              string
	traceFlags          uint32
	severityText        string
	severityNumber      uint8
	body                string
	scope               string
	attributes          string
	resource            string
	eventName           string
}

type resourceRow struct {
	labels              string
	fingerprint         string
	seenAtTSBucketStart int64
}

type batch struct {
	logs      []logRow
	resources []resourceRow
}

// batchBuilder converts plog.Logs into rows. Resources already registered in the exporter resource set are skipped.
type batchBuilder struct {
	exp          *auditExporter
	batch        batch
	fingerprints map[int64]map[string]string
	nowMilli     int64
}

type auditExporter struct {
	logger      *zap.Logger
	cfg         *Config
	conn        driver.Conn
	telemetry   *metadata.TelemetryBuilder
	tableAttr   map[string]metric.RecordOption
	resourceSet *timebucketedset.Set
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

	return &auditExporter{
		logger:      set.Logger,
		cfg:         cfg,
		telemetry:   telemetry,
		tableAttr:   tableAttr,
		resourceSet: resourceSet,
	}, nil
}

func (e *auditExporter) start(ctx context.Context, _ component.Host) error {
	options, err := e.cfg.buildClickHouseOptions()
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

	return nil
}

func (e *auditExporter) shutdown(_ context.Context) error {
	e.resourceSet.Shutdown()
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

	var resourceBatch, logsBatch driver.Batch

	if len(b.resources) > 0 {
		resourceBatch, err = e.conn.PrepareBatch(ctx, insertLogsResourceSQL, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("failed to prepare batch for %s: %w", distributedLogsResourceTable, err)
		}
		defer func() { _ = resourceBatch.Close() }()

		for _, r := range b.resources {
			if err := resourceBatch.Append(r.labels, r.fingerprint, r.seenAtTSBucketStart); err != nil {
				return fmt.Errorf("failed to append to %s: %w", distributedLogsResourceTable, err)
			}
		}
	}

	if len(b.logs) > 0 {
		logsBatch, err = e.conn.PrepareBatch(ctx, insertLogsSQL, driver.WithReleaseConnection())
		if err != nil {
			return fmt.Errorf("failed to prepare batch for %s: %w", distributedLogsTable, err)
		}
		defer func() { _ = logsBatch.Close() }()

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
				return fmt.Errorf("failed to append to %s: %w", distributedLogsTable, err)
			}
		}
	}

	errCh := make(chan error, 2)
	go func() { errCh <- e.flush(ctx, resourceBatch, distributedLogsResourceTable) }()
	go func() { errCh <- e.flush(ctx, logsBatch, distributedLogsTable) }()

	if err := errors.Join(<-errCh, <-errCh); err != nil {
		return err
	}

	e.resourceSet.Apply(func(yield func([]byte, int64) bool) {
		for _, r := range b.resources {
			if !yield([]byte(r.fingerprint), r.seenAtTSBucketStart*1000) {
				return
			}
		}
	})

	e.logger.Debug("inserted audit logs",
		zap.Int("logs", len(b.logs)),
		zap.Int("resources", len(b.resources)),
		zap.Duration("duration", time.Since(now)),
	)

	return nil
}

func (e *auditExporter) flush(ctx context.Context, batch driver.Batch, table string) error {
	if batch == nil {
		return nil
	}

	start := time.Now()
	err := batch.Send()
	e.telemetry.SignozclickhouseauditWriteDuration.Record(ctx, time.Since(start).Seconds(), e.tableAttr[table])
	return err
}

func (e *auditExporter) buildBatch(ld plog.Logs, now time.Time) (*batch, error) {
	b := &batchBuilder{
		exp:          e,
		fingerprints: map[int64]map[string]string{},
		nowMilli:     now.UnixMilli(),
	}

	resourceLogs := ld.ResourceLogs()
	for i := range resourceLogs.Len() {
		if err := b.addResourceLogs(resourceLogs.At(i), now); err != nil {
			return nil, err
		}
	}

	return &b.batch, nil
}

func (b *batchBuilder) addResourceLogs(rl plog.ResourceLogs, now time.Time) error {
	res := rl.Resource()
	resourceJSON, err := marshalJSON(map[string]any{"attributes": res.Attributes().AsRaw()})
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}
	labels, err := resourceLabels(res.Attributes())
	if err != nil {
		return fmt.Errorf("failed to marshal resource labels: %w", err)
	}

	scopeLogs := rl.ScopeLogs()
	for i := range scopeLogs.Len() {
		sl := scopeLogs.At(i)
		scope := sl.Scope()
		scopeJSON, err := marshalJSON(map[string]any{
			"name":       scope.Name(),
			"version":    scope.Version(),
			"attributes": scope.Attributes().AsRaw(),
		})
		if err != nil {
			return fmt.Errorf("failed to marshal scope: %w", err)
		}

		records := sl.LogRecords()
		for j := range records.Len() {
			record := records.At(j)

			ts := uint64(record.Timestamp())
			ots := uint64(record.ObservedTimestamp())
			if ots == 0 {
				ots = uint64(now.UnixNano())
			}
			if ts == 0 {
				ts = ots
			}

			id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
			if err != nil {
				return fmt.Errorf("failed to generate id: %w", err)
			}

			bucket := int64(ts/uint64(time.Second)) / bucketSeconds * bucketSeconds
			fp := b.fingerprint(bucket, labels, res.Attributes())

			body, err := bodyJSON(record.Body())
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
			attributes, err := marshalJSON(record.Attributes().AsRaw())
			if err != nil {
				return fmt.Errorf("failed to marshal attributes: %w", err)
			}

			b.batch.logs = append(b.batch.logs, logRow{
				tsBucketStart:       uint64(bucket),
				resourceFingerprint: fp,
				timestamp:           ts,
				observedTimestamp:   ots,
				id:                  id.String(),
				traceID:             utils.TraceIDToHexOrEmptyString(record.TraceID()),
				spanID:              utils.SpanIDToHexOrEmptyString(record.SpanID()),
				traceFlags:          uint32(record.Flags()),
				severityText:        record.SeverityText(),
				severityNumber:      uint8(record.SeverityNumber()),
				body:                body,
				scope:               scopeJSON,
				attributes:          attributes,
				resource:            resourceJSON,
				eventName:           record.EventName(),
			})
		}
	}

	return nil
}

func (b *batchBuilder) fingerprint(bucket int64, labels string, attrs pcommon.Map) string {
	byLabels, ok := b.fingerprints[bucket]
	if !ok {
		byLabels = map[string]string{}
		b.fingerprints[bucket] = byLabels
	}
	if fp, ok := byLabels[labels]; ok {
		return fp
	}

	fp := fingerprint.CalculateFingerprint(attrs.AsRaw(), fingerprint.AuditResourceHierarchy())
	byLabels[labels] = fp
	if write, _ := b.exp.resourceSet.Plan([]byte(fp), bucket*1000, b.nowMilli); write {
		b.batch.resources = append(b.batch.resources, resourceRow{labels: labels, fingerprint: fp, seenAtTSBucketStart: bucket})
	}

	return fp
}

// bodyJSON keeps a map body as is and wraps any other body as {"message": <body as string>}.
func bodyJSON(body pcommon.Value) (string, error) {
	if body.Type() == pcommon.ValueTypeMap {
		return marshalJSON(body.Map().AsRaw())
	}
	return marshalJSON(map[string]any{"message": body.AsString()})
}

func resourceLabels(attrs pcommon.Map) (string, error) {
	labels := make(map[string]string, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
	b, err := json.Marshal(labels)
	return string(b), err
}

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(utils.SanitizeJSON(v))
	return string(b), err
}
