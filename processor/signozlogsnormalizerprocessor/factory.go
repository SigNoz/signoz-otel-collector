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
		OpAMP:          OpAMPConfig{Position: PositionAfter},
		Body:           BodyConfig{Enabled: true, Fields: []string{"log", "msg"}},
		SeverityNumber: FieldConfig{Fields: []string{"severity_number", "severitynumber"}, Action: ActionCopy},
		SeverityText:   FieldConfig{Fields: []string{"severity_text", "severitytext", "severity", "level", "log.level", "log_level", "loglevel", "levelname", "lvl"}, Action: ActionCopy},
		TraceID:        FieldConfig{Fields: []string{"trace_id", "traceid", "trace.id"}, Action: ActionCopy},
		SpanID:         FieldConfig{Fields: []string{"span_id", "spanid", "span.id"}, Action: ActionCopy},
		ScopeName:      FieldConfig{Fields: []string{"scope.name", "scope_name", "scopename"}, Action: ActionCopy},
		ScopeVersion:   FieldConfig{Fields: []string{"scope.version", "scope_version", "scopeversion"}, Action: ActionCopy},
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
