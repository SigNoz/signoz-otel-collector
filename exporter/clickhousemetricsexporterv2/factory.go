package clickhousemetricsexporterv2

import (
	"context"
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "clickhousemetricswritev2"
)

var (
	writeLatencyMillis = stats.Int64("exporter_db_write_latency", "Time taken (in millis) for exporter to write batch", "ms")
	exporterKey        = tag.MustNewKey("exporter")
	tableKey           = tag.MustNewKey("table")
)

// NewFactory creates a new ClickHouse Metrics exporter.
func NewFactory() exporter.Factory {

	writeLatencyDistribution := view.Distribution(100, 500, 750, 1000, 1500, 2000, 2500, 3000, 4000, 8000, 16000, 32000, 64000)
	writeLatencyView := &view.View{
		Name:        "exporter_db_write_latency",
		Measure:     writeLatencyMillis,
		Description: writeLatencyMillis.Description(),
		TagKeys:     []tag.Key{exporterKey, tableKey},
		Aggregation: writeLatencyDistribution,
	}

	view.Register(writeLatencyView)
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelUndefined))
}

func createMetricsExporter(ctx context.Context, set exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {

	chCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	connOptions, err := clickhouse.ParseDSN(chCfg.DSN)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, err
	}

	chExporter, err := NewClickHouseExporter(
		WithConfig(chCfg),
		WithConn(conn),
		WithLogger(set.Logger),
		WithEnableExpHist(chCfg.EnableExpHist),
	)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		chExporter.PushMetrics,
		exporterhelper.WithTimeout(chCfg.TimeoutSettings),
		exporterhelper.WithQueue(chCfg.QueueSettings),
		exporterhelper.WithRetry(chCfg.BackOffConfig),
		exporterhelper.WithStart(chExporter.Start),
		exporterhelper.WithShutdown(chExporter.Shutdown),
	)

	if err != nil {
		return nil, err
	}

	return exporter, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		DSN:             "tcp://localhost:9000",
		EnableExpHist:   false,
		Database:        "signoz_metrics",
		SamplesTable:    "distributed_samples_v4",
		TimeSeriesTable: "distributed_time_series_v4",
		ExpHistTable:    "distributed_exp_hist",
		MetadataTable:   "distributed_metadata",
	}
}
