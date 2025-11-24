package config

import "github.com/spf13/cobra"

var (
	Clickhouse       clickhouse
	MigrateSyncCheck migrateSyncCheck
)

type clickhouse struct {
	DSN string
}

func (cfg *clickhouse) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.DSN, "clickhouse-dsn", "tcp://0.0.0.0:9001", "the dsn for clickhouse connection")
}

type migrateSyncCheck struct {
	Timeout string
}

func (cfg *migrateSyncCheck) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.Timeout, "timeout", "10s", "The timeout for sync check")
}
