package metadataexporter

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

// LogsMetadataWriter is the extension point for all logs metadata writing
// responsibilities in the metadata exporter. Each implementation owns exactly
// one concern (one table or one logical unit of work). New responsibilities are
// added by implementing this interface and appending to
// metadataExporter.logsMetadataWriters in the constructor — existing writers
// are never touched.
type LogsMetadataWriter interface {
	ProcessMetadata(ctx context.Context, ld plog.Logs) error
}
