package signozclickhouseauditexporter

import (
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the ClickHouse audit logs exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	DSN                          string                                                   `mapstructure:"dsn"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}

	if _, err := clickhouse.ParseDSN(cfg.DSN); err != nil {
		return fmt.Errorf("invalid dsn: %w", err)
	}

	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		if err := qc.Validate(); err != nil {
			return err
		}
	}

	if err := cfg.TimeoutConfig.Validate(); err != nil {
		return err
	}

	return cfg.BackOffConfig.Validate()
}

func (cfg *Config) clickHouseOptions() (*clickhouse.Options, error) {
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	if options.Settings == nil {
		options.Settings = clickhouse.Settings{}
	}
	// JSON columns are sent as text; keep string values that look like dates as strings.
	options.Settings["type_json_skip_duplicated_paths"] = 1
	options.Settings["input_format_try_infer_datetimes"] = 0
	options.Settings["input_format_try_infer_dates"] = 0

	maxIdleConns := 1
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		maxIdleConns = qc.NumConsumers + 1
	}
	if options.MaxIdleConns < maxIdleConns {
		options.MaxIdleConns = maxIdleConns
		options.MaxOpenConns = maxIdleConns + 5
	}

	return options, nil
}
