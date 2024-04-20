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

// NewFactory creates Kafka receiver factory.
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
	// TODO(Raj): Is this needed here?
	err := rCfg.Validate()
	if err != nil {
		return nil, err
	}

	scrapeIntervalSeconds := rCfg.ScrapeIntervalSeconds
	if scrapeIntervalSeconds == 0 {
		scrapeIntervalSeconds = defaultScrapeIntervalSeconds
	}

	db, err := newClickhouseClient(rCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("couldn't create clickhouse client: %w", err)
	}

	return &systemTablesReceiver{
		db:                    db,
		scrapeIntervalSeconds: scrapeIntervalSeconds,
		// TODO(Raj): is params.Logger always provided to be non null?
		logger: params.Logger,
	}, nil

}
