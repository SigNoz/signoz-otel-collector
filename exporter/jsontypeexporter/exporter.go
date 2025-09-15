package jsontypeexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type jsonTypeExporter struct {
	config *Config
	logger *zap.Logger
}

func newExporter(cfg Config, set exporter.Settings) (*jsonTypeExporter, error) {
	return &jsonTypeExporter{
		config: &cfg,
		logger: set.Logger,
	}, nil
}

func (e *jsonTypeExporter) start(ctx context.Context, host component.Host) error {
	e.logger.Info("JSON Type exporter started")
	return nil
}

func (e *jsonTypeExporter) shutdown(ctx context.Context) error {
	e.logger.Info("JSON Type exporter shutdown")
	return nil
}

func (e *jsonTypeExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	e.logger.Debug("Processing logs", zap.Int("log_record_count", ld.LogRecordCount()))
	// TODO: Implement logs processing logic
	return nil
}
