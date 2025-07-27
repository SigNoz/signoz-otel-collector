package signozclickhousemeter

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for ClickHouse Metrics exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	BackOffConfig                configretry.BackOffConfig       `mapstructure:"retry_on_failure"`
	QueueBatchConfig             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	DSN                          string                          `mapstructure:"dsn"`
	Database                     string                          `mapstructure:"database"`
	SamplesTable                 string                          `mapstructure:"samples_table"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if err := cfg.QueueBatchConfig.Validate(); err != nil {
		return err
	}

	if err := cfg.TimeoutConfig.Validate(); err != nil {
		return err
	}

	if err := cfg.BackOffConfig.Validate(); err != nil {
		return err
	}

	return nil
}
