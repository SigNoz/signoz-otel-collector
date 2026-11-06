package signozmetricreductionrulesextension

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/SigNoz/signoz-otel-collector/extension/signozmetricreductionrulesextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		DSN:          "tcp://localhost:9000",
		Database:     "signoz_metrics",
		PollInterval: 45 * time.Second,
		RulesTable:   "distributed_metric_reduction_rules",
	}
}

func createExtension(
	_ context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	rulesCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type %T for %s extension", cfg, metadata.Type)
	}

	connOptions, err := clickhouse.ParseDSN(rulesCfg.DSN)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, err
	}

	return newExtension(rulesCfg, set.TelemetrySettings, conn)
}
