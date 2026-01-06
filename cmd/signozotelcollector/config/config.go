package config

import (
	"time"

	signozcolFeatureGate "github.com/SigNoz/signoz-otel-collector/featuregate"
	"github.com/spf13/cobra"
	otelcolFeatureGate "go.opentelemetry.io/collector/featuregate"
)

var (
	Collector        collector
	Clickhouse       clickhouse
	MigrateReady     migrateReady
	MigrateBootstrap migrateBootstrap
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
	DSN         string
	Cluster     string
	Replication bool
}

func (cfg *clickhouse) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfg.DSN, "clickhouse-dsn", "tcp://0.0.0.0:9001", "DSN for clickhouse connection")
	cmd.PersistentFlags().StringVar(&cfg.Cluster, "clickhouse-cluster", "cluster", "Name of the clickhouse cluster to connect")
	cmd.PersistentFlags().BoolVar(&cfg.Replication, "clickhouse-replication", false, "Set true if replication is enabled in the clickhouse cluster")
}

type migrateReady struct {
	Timeout time.Duration
}

func (cfg *migrateReady) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().DurationVar(&cfg.Timeout, "timeout", time.Duration(10*time.Second), "Timeout for migrate ready operation")
}

type migrateSyncCheck struct {
	Timeout time.Duration
}

func (cfg *migrateSyncCheck) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().DurationVar(&cfg.Timeout, "timeout", time.Duration(10*time.Second), "Timeout for sync check operation")
}

type migrateBootstrap struct {
	Timeout time.Duration
}

func (cfg *migrateBootstrap) RegisterFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().DurationVar(&cfg.Timeout, "timeout", time.Duration(15*time.Minute), "Timeout for bootstrap operation")
}
