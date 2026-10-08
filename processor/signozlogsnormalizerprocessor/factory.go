package signozlogsnormalizerprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsnormalizerprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Body: BodyConfig{Enabled: true, MessageFields: []string{"log", "msg"}},
		Fields: FieldsConfig{
			Enabled:        true,
			SeverityNumber: []string{"severity_number", "severitynumber"},
			SeverityText:   []string{"severity_text", "severitytext", "severity", "level", "log.level", "log_level", "loglevel", "levelname", "lvl"},
			TraceID:        []string{"trace_id", "traceid", "trace.id"},
			SpanID:         []string{"span_id", "spanid", "span.id"},
			ScopeName:      []string{"scope.name", "scope_name", "scopename"},
			ScopeVersion:   []string{"scope.version", "scope_version", "scopeversion"},
		},
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type %T for %s processor", cfg, metadata.Type)
	}
	proc, err := newNormalizeProcessor(set.TelemetrySettings, pCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(
		ctx, set, cfg, nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithShutdown(proc.telemetry.shutdown),
	)
}
