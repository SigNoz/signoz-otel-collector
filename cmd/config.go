package main

import (
	"flag"
	"fmt"
	"time"

	cacheStrategies "github.com/SigNoz/signoz-otel-collector/pkg/cache/strategies"

	storageStrategies "github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/featuregate"
)

type storageConfig struct {
	strategy storageStrategies.Strategy
	host     string
	port     int
	user     string
	password string
	database string
}

func (sc *storageConfig) registerFlags(cmd *cobra.Command) {
	sc.strategy = storageStrategies.Off
	cmd.Flags().Var(&sc.strategy, "storage-strategy", fmt.Sprintf("Strategy to use for storage, allowed values are: %v", storageStrategies.AllowedStrategies()))
	cmd.Flags().StringVar(&sc.host, "postgres-host", "0.0.0.0", "Host of postgres")
	cmd.Flags().IntVar(&sc.port, "postgres-port", 5432, "Port of postgres")
	cmd.Flags().StringVar(&sc.user, "postgres-user", "postgres", "User of postgres")
	cmd.Flags().StringVar(&sc.password, "postgres-password", "password", "Password of postgres")
	cmd.Flags().StringVar(&sc.database, "postgres-database", "collector", "Database of postgres")
}

type adminHttpConfig struct {
	bindAddress string
	gracePeriod time.Duration
}

func (ahc *adminHttpConfig) registerFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&ahc.bindAddress, "admin-http-address", "0.0.0.0:8001", "Listen host:port for all admin http endpoints")
	cmd.Flags().DurationVar(&ahc.gracePeriod, "admin-http-grace-period", 5*time.Second, "Time to wait after an interrupt received for the admin http server")
}

type collectorConfig struct {
	config        string
	managerConfig string
	copyPath      string
}

func (cc *collectorConfig) registerFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cc.config, "config", "", "File path for the collector configuration")
	cmd.Flags().StringVar(&cc.managerConfig, "manager-config", "", "File path for the agent manager configuration")
	cmd.Flags().StringVar(&cc.copyPath, "copy-path", "/etc/otel/signozcol-config.yaml", "File path for the copied collector configuration")

	flagSet := new(flag.FlagSet)
	// TODO: Remove featuregate package from this repository is the below works
	flagSet.Var(featuregate.NewFlag(featuregate.GlobalRegistry()), "feature-gates",
		"Comma-delimited list of feature gate identifiers. Prefix with '-' to disable the feature. '+' or no prefix will enable the feature.")
	cmd.Flags().AddGoFlagSet(flagSet)
}

type cacheConfig struct {
	strategy cacheStrategies.Strategy
	host     string
	port     int
}

func (cc *cacheConfig) registerFlags(cmd *cobra.Command) {
	cc.strategy = cacheStrategies.Off
	cmd.Flags().Var(&cc.strategy, "cache-strategy", fmt.Sprintf("Strategy to use for cache, allowed values are: %v", cacheStrategies.AllowedStrategies()))
	cmd.Flags().StringVar(&cc.host, "redis-host", "0.0.0.0", "Host of redis")
	cmd.Flags().IntVar(&cc.port, "redis-port", 6379, "Port of redis")
}

type migrateConfig struct {
	hclPath       string
	migrationsDir string
}

func (mc *migrateConfig) registerFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&mc.hclPath, "hcl-path", "./pkg/storage/migrations/atlas.hcl", "Path to the atlas hcl configuration flag")
	cmd.Flags().StringVar(&mc.migrationsDir, "migrations-dir", "./pkg/storage/migrations/migrations", "Path to the directory containing all migration files")
}
