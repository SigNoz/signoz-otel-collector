package config

import (
	signozcolFeatureGate "github.com/SigNoz/signoz-otel-collector/featuregate"
	"github.com/spf13/cobra"
	otelcolFeatureGate "go.opentelemetry.io/collector/featuregate"
)

var (
	Collector        collector
	Clickhouse       clickhouse
	MigrateReady     migrateReady
	MigrateSyncCheck migrateSyncCheck
)

type collector struct {
	Config        string
	ManagerConfig string
	CopyPath      string
}

func (cfg *collector) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.Config, "config", "", "File path for the collector configuration")
	cmd.PersistentFlags().StringVar(&cfg.ManagerConfig, "manager-config", "", "File path for the agent manager configuration")
	cmd.PersistentFlags().StringVar(&cfg.CopyPath, "copy-path", "/etc/otel/signozcol-config.yaml", "File path for the copied collector configuration")
	cmd.PersistentFlags().Var(signozcolFeatureGate.NewFlag(otelcolFeatureGate.GlobalRegistry()), "feature-gates",
		"Comma-delimited list of feature gate identifiers. Prefix with '-' to disable the feature. '+' or no prefix will enable the feature.")
}

type clickhouse struct {
	DSN      string
	Shards   uint64
	Replicas uint64
	Cluster  string
	Version  string
}

func (cfg *clickhouse) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.DSN, "clickhouse-dsn", "tcp://0.0.0.0:9001", "DSN for clickhouse connection")
	cmd.PersistentFlags().StringVar(&cfg.Cluster, "clickhouse-cluster", "cluster", "Name of the clickhouse cluster to connect")
}

type migrateReady struct {
	Timeout string
}

func (cfg *migrateReady) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.Timeout, "timeout", "10s", "Timeout for migrate ready operation")
}

type migrateSyncCheck struct {
	Timeout string
}

func (cfg *migrateSyncCheck) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.Timeout, "timeout", "10s", "Timeout for sync check operation")
}
