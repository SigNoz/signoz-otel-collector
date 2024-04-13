package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/featuregate"
)

type storageConfig struct {
	strategy strategies.Strategy
	host     string
	port     int
	user     string
	password string
	database string
}

func (sc *storageConfig) registerFlags(cmd *cobra.Command) {
	sc.strategy = strategies.Off
	cmd.Flags().Var(&sc.strategy, "strategy", fmt.Sprintf("Strategy to use for storage, allowed Values are: %v", strategies.AllowedStrategies()))
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
