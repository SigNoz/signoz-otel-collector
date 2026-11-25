package signozclickhouseauditexporter

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	TimeoutSettings exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	BackOffConfig   configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueSettings   configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	DSN string `mapstructure:"dsn"`
}

var _ component.Config = (*Config)(nil)

var (
	errConfigNoDSN      = errors.New("dsn must be specified")
	errConfigInvalidDSN = errors.New("dsn must be url format")
)

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		DSN:             "tcp://localhost:9000",
	}
}

func (cfg *Config) Validate() (err error) {
	if cfg.DSN == "" {
		err = errors.Join(err, errConfigNoDSN)
	}

	dsn, e := cfg.buildDSN()
	if e != nil {
		err = errors.Join(err, e)
	}

	if _, e := clickhouse.ParseDSN(dsn); e != nil {
		err = errors.Join(err, e)
	}

	return err
}

// buildDSN adds the exporter defaults to the DSN; values already set in the DSN win.
func (cfg *Config) buildDSN() (string, error) {
	dsnURL, err := url.Parse(cfg.DSN)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errConfigInvalidDSN, err.Error())
	}

	queryParams := dsnURL.Query()

	if !queryParams.Has("async_insert") {
		queryParams.Set("async_insert", "true")
	}

	if !queryParams.Has("compress") {
		queryParams.Set("compress", "lz4")
	}

	// JSON columns are sent as text; keep string values that look like dates as strings.
	for name, value := range map[string]string{
		"type_json_skip_duplicated_paths":  "1",
		"input_format_try_infer_datetimes": "0",
		"input_format_try_infer_dates":     "0",
	} {
		if !queryParams.Has(name) {
			queryParams.Set(name, value)
		}
	}

	dsnURL.RawQuery = queryParams.Encode()

	return dsnURL.String(), nil
}

func (cfg *Config) buildClickHouseOptions() (*clickhouse.Options, error) {
	dsn, err := cfg.buildDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN from config: %w", err)
	}

	opt, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	return opt, nil
}
