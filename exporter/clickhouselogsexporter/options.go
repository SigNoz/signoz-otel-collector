package clickhouselogsexporter

import (
	"fmt"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type LogExporterOption func(*clickhouseLogsExporter)

func WithClickHouseClient(client driver.Conn) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.db = client
	}
}

func WithLogger(logger *zap.Logger) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.logger = logger
	}
}

func WithNewUsageCollector(id uuid.UUID, db driver.Conn) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.usageCollector = usage.NewUsageCollector(
			id,
			e.db,
			usage.Options{
				ReportingInterval: usage.DefaultCollectionInterval,
			},
			"signoz_logs",
			UsageExporter,
			e.logger,
		)
		// TODO: handle error
		_ = e.usageCollector.Start()
		e.id = id
	}
}

func WithMeter(meter metric.Meter) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		durationHistogram, err := meter.Float64Histogram(
			"exporter_db_write_latency",
			metric.WithDescription("Time taken to write data to ClickHouse"),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
		)
		if err != nil {
			panic(fmt.Errorf("error creating duration histogram: %w", err))
		}
		e.durationHistogram = durationHistogram
	}
}

func WithKeysCache(keysCache *ttlcache.Cache[string, struct{}]) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.keysCache = keysCache
	}
}

func WithRFCache(rfCache *ttlcache.Cache[string, struct{}]) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.rfCache = rfCache
	}
}

func WithConcurrency(concurrency int) LogExporterOption {
	return func(e *clickhouseLogsExporter) {
		e.limiter = make(chan struct{}, concurrency)
	}
}
