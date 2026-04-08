package signozclickhouseauditexporter

import (
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the signoz audit exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// DSN is the ClickHouse server Data Source Name.
	DSN string `mapstructure:"dsn"`
}

func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}

	if _, err := cfg.buildClickHouseOptions(); err != nil {
		return fmt.Errorf("invalid dsn: %w", err)
	}

	return nil
}

// buildClickHouseOptions parses the DSN and applies connection tuning.
func (cfg *Config) buildClickHouseOptions() (*clickhouse.Options, error) {
	options, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	if options.Settings == nil {
		options.Settings = clickhouse.Settings{}
	}
	options.Settings["type_json_skip_duplicated_paths"] = 1

	maxIdleConnections := 1
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		maxIdleConnections = qc.NumConsumers + 1
	}
	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}

	return options, nil
}
