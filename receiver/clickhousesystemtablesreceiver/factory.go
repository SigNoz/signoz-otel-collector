package clickhousesystemtablesreceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	defaultScrapeIntervalSeconds = 10
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		"clickhousesystemtablesreceiver",
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha))

}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	rCfg := cfg.(*Config)

	scrapeIntervalSeconds := rCfg.QueryLogScrapeConfig.ScrapeIntervalSeconds
	if scrapeIntervalSeconds == 0 {
		scrapeIntervalSeconds = defaultScrapeIntervalSeconds
	}

	db, err := newClickhouseClient(rCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("couldn't create clickhouse client: %w", err)
	}

	chQuerrier := newClickhouseQuerrier(db)

	logger := params.Logger
	if logger == nil {
		return nil, fmt.Errorf("logsReceiver must be provided with a logger")
	}

	return &systemTablesReceiver{
		nextConsumer:          consumer,
		clickhouse:            chQuerrier,
		scrapeIntervalSeconds: scrapeIntervalSeconds,
		scrapeDelaySeconds:    rCfg.QueryLogScrapeConfig.MinScrapeDelaySeconds,
		logger:                params.Logger,
	}, nil

}
