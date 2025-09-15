package jsontypeexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/SigNoz/signoz-otel-collector/exporter/jsontypeexporter/internal/metadata"
)

// NewFactory creates JSON Type exporter factory.
func NewFactory() exporter.Factory {
	f := &jsonTypeExporterFactory{}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
		QueueBatchConfig: exporterhelper.NewDefaultQueueConfig(),
		OutputPath:       "./output.json",
	}
}

type jsonTypeExporterFactory struct {
}

func (f *jsonTypeExporterFactory) createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := *(cfg.(*Config)) // Clone the config
	exp, err := newExporter(oCfg, set)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(
		ctx,
		set,
		&oCfg,
		exp.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithQueue(oCfg.QueueBatchConfig),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown))
}
