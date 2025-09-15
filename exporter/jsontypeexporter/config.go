package jsontypeexporter

import (
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for JSON Type exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	QueueBatchConfig             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// OutputPath is the path where the JSON files will be written.
	OutputPath string `mapstructure:"output_path"`
}
