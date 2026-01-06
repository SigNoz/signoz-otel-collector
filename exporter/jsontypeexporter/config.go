package jsontypeexporter

import (
	"errors"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for JSON Type exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`                                 // squash ensures fields are correctly decoded in embedded struct.
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// DSN is the ClickHouse server Data Source Name.
	DSN string `mapstructure:"dsn"`

	// MaxDepthTraverse is the maximum depth of the JSON object to traverse.
	MaxDepthTraverse *int `mapstructure:"max_depth_traverse"`
	// MaxArrayElementsAllowed is the maximum number of elements in an array allowed or to be skipped.
	MaxArrayElementsAllowed *int `mapstructure:"max_array_elements_allowed"`

	// FailOnError determines whether to return errors or only log them.
	// When false (default), errors are logged but not returned, allowing the exporter to continue processing.
	// When true, errors are returned, which is useful for testing and debugging.
	FailOnError bool `mapstructure:"fail_on_error"`
}

var _ component.Config = (*Config)(nil)

const (
	defaultMaxDepthTraverse        = 22
	defaultMaxArrayElementsAllowed = 100
	defaultMaxKeysAtLevel          = 1024
)

var (
	errConfigNoDSN = errors.New("dsn must be specified")
)

// Validate validates the JSON Type exporter configuration.
func (cfg *Config) Validate() error {
	var err error
	if cfg.DSN == "" {
		err = multierr.Append(err, errConfigNoDSN)
	}
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		if validationErr := qc.Validate(); validationErr != nil {
			err = multierr.Append(err, validationErr)
		}
	}
	if validationErr := cfg.TimeoutConfig.Validate(); validationErr != nil {
		err = multierr.Append(err, validationErr)
	}
	if validationErr := cfg.BackOffConfig.Validate(); validationErr != nil {
		err = multierr.Append(err, validationErr)
	}

	if cfg.MaxDepthTraverse == nil {
		cfg.MaxDepthTraverse = utils.ToPointer(defaultMaxDepthTraverse)
	}
	if cfg.MaxArrayElementsAllowed == nil {
		cfg.MaxArrayElementsAllowed = utils.ToPointer(defaultMaxArrayElementsAllowed)
	}

	if *cfg.MaxDepthTraverse < 1 {
		err = multierr.Append(err, errors.New("max_depth_traverse must be greater than 0"))
	}
	if *cfg.MaxArrayElementsAllowed < 1 {
		err = multierr.Append(err, errors.New("max_array_elements_allowed must be greater than 0"))
	}
	return err
}
