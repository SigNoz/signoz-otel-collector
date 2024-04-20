package clickhousesystemtablesreceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"

import (
	"errors"

	"go.uber.org/multierr"
)

// TODO(Raj): Add config_test

type Config struct {
	DSN                   string `mapstructure:"dsn"`
	ScrapeIntervalSeconds uint64 `mapstructure:"scrape_interval_seconds"`
}

func (cfg *Config) Validate() (err error) {
	if cfg.DSN == "" {
		err = multierr.Append(err, errors.New("dsn must be specified"))
	}
	return err
}
