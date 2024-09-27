package clickhousemetricsexporterv2

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for ClickHouse Metrics exporter.
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`

	DSN string `mapstructure:"dsn"`

	EnableExpHist bool `mapstructure:"enable_exp_hist"`

	Database        string
	SamplesTable    string
	TimeSeriesTable string
	ExpHistTable    string
	MetadataTable   string
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if err := cfg.QueueSettings.Validate(); err != nil {
		return err
	}

	if err := cfg.TimeoutSettings.Validate(); err != nil {
		return err
	}

	if err := cfg.BackOffConfig.Validate(); err != nil {
		return err
	}

	return nil
}
