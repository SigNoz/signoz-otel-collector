package jsontypeexporter

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for JSON Type exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	BackOffConfig                configretry.BackOffConfig       `mapstructure:"retry_on_failure"`
	QueueBatchConfig             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// DSN is the ClickHouse server Data Source Name.
	DSN string `mapstructure:"dsn"`
}

var _ component.Config = (*Config)(nil)

var (
	errConfigNoDSN = errors.New("dsn must be specified")
)

// Validate validates the JSON Type exporter configuration.
func (cfg *Config) Validate() error {
	var err error
	if cfg.DSN == "" {
		err = multierr.Append(err, errConfigNoDSN)
	}
	if validationErr := cfg.QueueBatchConfig.Validate(); validationErr != nil {
		err = multierr.Append(err, validationErr)
	}
	if validationErr := cfg.TimeoutConfig.Validate(); validationErr != nil {
		err = multierr.Append(err, validationErr)
	}
	if validationErr := cfg.BackOffConfig.Validate(); validationErr != nil {
		err = multierr.Append(err, validationErr)
	}
	return err
}
