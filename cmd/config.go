package main

import (
	"fmt"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"github.com/spf13/cobra"
)

type storageConfig struct {
	strategy storage.Strategy
	host     string
	port     int
	user     string
	password string
	database string
}

func (sc *storageConfig) registerFlags(cmd *cobra.Command) {
	sc.strategy = storage.Off
	cmd.Flags().Var(&sc.strategy, "strategy", fmt.Sprintf("Strategy to use for storage, allowed Values are: %v", storage.AllowedStrategies()))
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
