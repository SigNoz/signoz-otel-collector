package clickhousesystemtablesreceiver

import (
	"errors"

	"go.uber.org/multierr"
)

type QueryLogScrapeConfig struct {
	ScrapeIntervalSeconds uint32 `mapstructure:"scrape_interval_seconds"`

	// Must be configured to a value greater than flush_interval_milliseconds setting for query_log
	// For details see https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings#query-log
	MinScrapeDelaySeconds uint32 `mapstructure:"min_scrape_delay_seconds"`
}

type Config struct {
	DSN string `mapstructure:"dsn"`

	// Cluster name to use for scraping query log from clustered deployments.
	// If ClusterName is specified, the scrape will target `clusterAllReplicas(cluster_name, system.query_log)`
	ClusterName string `mapstructure:"cluster_name"`

	QueryLogScrapeConfig QueryLogScrapeConfig `mapstructure:"query_log_scrape_config"`
}

func (cfg *Config) Validate() (err error) {
	if cfg.DSN == "" {
		err = multierr.Append(err, errors.New("dsn must be specified"))
	}

	if cfg.QueryLogScrapeConfig.MinScrapeDelaySeconds == 0 {
		err = multierr.Append(err, errors.New("query_log_scrape_config.scrape_delay_seconds must be set to a value greater than flush_interval_milliseconds setting for clickhouse query_log table"))
	}

	return err
}
