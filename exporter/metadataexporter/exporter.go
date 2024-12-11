package metadataexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tagValueCountFromDB struct {
	tagDataType         string
	stringTagValueCount uint64
	numberValueCount    uint64
}

type metadataExporter struct {
	cfg Config
	set exporter.Settings
}

func newMetadataExporter(cfg Config, set exporter.Settings) (*metadataExporter, error) {
	return &metadataExporter{cfg: cfg, set: set}, nil
}

func (e *metadataExporter) Start(_ context.Context, host component.Host) error {
	return nil
}

func (e *metadataExporter) Shutdown(_ context.Context) error {
	return nil
}

func (e *metadataExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func (e *metadataExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (e *metadataExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
