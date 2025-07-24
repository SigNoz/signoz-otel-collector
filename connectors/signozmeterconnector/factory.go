package signozmeterconnector

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozmeterconnector/internal/metadata"
)

const (
	defaultMetricsFlushInterval = time.Hour * 1
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetrics, metadata.TracesToMetricsStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		connector.WithLogsToMetrics(createLogsToMetrics, metadata.LogsToMetricsStability),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{
		Dimensions: []Dimension{
			{
				Name: "service.name",
			},
			{
				Name: "deployment.environment",
			},
			{
				Name: "host.name",
			},
		},
		MetricsFlushInterval: defaultMetricsFlushInterval,
	}
}

// createTracesToMetrics creates a traces to metrics connector based on provided config.
func createTracesToMetrics(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	mu.Lock()
	defer mu.Unlock()
	oCfg := cfg.(*Config)

	connector := connectors[oCfg]

	if connector == nil {
		c, err := newConnector(params.Logger, cfg)
		if err != nil {
			return nil, err
		}
		c.metricsConsumer = nextConsumer
		connectors[oCfg] = c
	}

	return connectors[oCfg], nil
}

// createLogsToMetrics creates a logs to metrics connector based on provided config.
func createLogsToMetrics(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Logs, error) {
	mu.Lock()
	defer mu.Unlock()
	oCfg := cfg.(*Config)

	connector := connectors[oCfg]

	if connector == nil {
		c, err := newConnector(params.Logger, cfg)
		if err != nil {
			return nil, err
		}
		c.metricsConsumer = nextConsumer
		connectors[oCfg] = c
	}

	return connectors[oCfg], nil
}

// createMetricsToMetrics creates a metrics to metrics connector based on provided config.
func createMetricsToMetrics(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Metrics, error) {
	mu.Lock()
	defer mu.Unlock()
	oCfg := cfg.(*Config)

	connector := connectors[oCfg]
	if connector == nil {
		c, err := newConnector(params.Logger, cfg)
		if err != nil {
			return nil, err
		}
		c.metricsConsumer = nextConsumer
		connectors[oCfg] = c
	}

	return connectors[oCfg], nil
}

var mu sync.Mutex
var connectors = map[*Config]*meterConnector{}
