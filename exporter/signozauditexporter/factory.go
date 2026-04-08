package signozauditexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozauditexporter/internal/metadata"
)

const (
	databaseName = "signoz_audit"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
		QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)

	client, err := newClickhouseClient(set.Logger, c)
	if err != nil {
		return nil, fmt.Errorf("cannot configure signoz audit exporter: %w", err)
	}

	keysCache := ttlcache.New(
		ttlcache.WithTTL[string, struct{}](240*time.Minute),
		ttlcache.WithCapacity[string, struct{}](50000),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
	)
	go keysCache.Start()

	rfCache := ttlcache.New(
		ttlcache.WithTTL[string, struct{}](distributedLogsResourceSeconds*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	meter := set.MeterProvider.Meter(metadata.ScopeName)
	exp, err := newExporter(set.Logger, c, client, keysCache, rfCache, meter)
	if err != nil {
		return nil, fmt.Errorf("cannot configure signoz audit exporter: %w", err)
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exp.pushLogsData,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
		exporterhelper.WithTimeout(c.TimeoutConfig),
		exporterhelper.WithQueue(c.QueueBatchConfig),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func newClickhouseClient(_ *zap.Logger, cfg *Config) (clickhouse.Conn, error) {
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("cannot parse DSN: %w", err)
	}

	if options.Settings == nil {
		options.Settings = clickhouse.Settings{}
	}
	options.Settings["type_json_skip_duplicated_paths"] = 1

	maxIdleConnections := 1
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		maxIdleConnections = qc.NumConsumers + 1
	}
	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("cannot open clickhouse connection: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("cannot ping clickhouse: %w", err)
	}

	return db, nil
}
