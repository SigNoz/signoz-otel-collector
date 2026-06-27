package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver/internal/metadata"
)

const (
	defaultScrapeIntervalSeconds     = 10
	defaultMetricsCollectionInterval = 30 * time.Second
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		SystemTablesMetrics: SystemTablesMetricsConfig{
			CollectionInterval:   defaultMetricsCollectionInterval,
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		},
	}
}

func createLogsReceiver(
	ctx context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	rCfg := cfg.(*Config)

	scrapeIntervalSeconds := rCfg.QueryLogScrapeConfig.ScrapeIntervalSeconds
	if scrapeIntervalSeconds == 0 {
		scrapeIntervalSeconds = defaultScrapeIntervalSeconds
	}

	logger := params.Logger
	if logger == nil {
		return nil, fmt.Errorf("logger must be provided")
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              "clickhouse",
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	return &systemTablesReceiver{
		nextConsumer:          consumer,
		config:                rCfg,
		scrapeIntervalSeconds: scrapeIntervalSeconds,
		scrapeDelaySeconds:    rCfg.QueryLogScrapeConfig.MinScrapeDelaySeconds,
		logger:                params.Logger,
		obsrecv:               obsrecv,
	}, nil

}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)

	metricsCfg := &rCfg.SystemTablesMetrics
	if metricsCfg.CollectionInterval <= 0 {
		metricsCfg.CollectionInterval = defaultMetricsCollectionInterval
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              "clickhouse",
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	return &systemTablesMetricsReceiver{
		config:       rCfg,
		metricsCfg:   metricsCfg,
		mb:           metadata.NewMetricsBuilder(metricsCfg.MetricsBuilderConfig, params),
		nextConsumer: consumer,
		logger:       params.Logger,
		obsrecv:      obsrecv,
	}, nil
}
