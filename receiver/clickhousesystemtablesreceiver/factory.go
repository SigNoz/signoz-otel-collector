package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	defaultScrapeIntervalSeconds = 10
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType("clickhousesystemtablesreceiver"),
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
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

	db, err := newClickhouseClient(ctx, rCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("couldn't create clickhouse client: %w", err)
	}

	chQuerrier := newClickhouseQuerrier(db, rCfg.ClusterName)

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
		clickhouse:            chQuerrier,
		scrapeIntervalSeconds: scrapeIntervalSeconds,
		scrapeDelaySeconds:    rCfg.QueryLogScrapeConfig.MinScrapeDelaySeconds,
		logger:                params.Logger,
		obsrecv:               obsrecv,
	}, nil

}
