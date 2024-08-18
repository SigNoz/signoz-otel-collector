package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	schema_migrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogger() *zap.Logger {
	// Always verbose logging for schema migrator
	config := zap.NewDevelopmentConfig()
	config.Encoding = "json"
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger %v", err)
	}
	return logger
}

func main() {
	cmd := &cobra.Command{
		Use:   "signoz-schema-migrator",
		Short: "Signoz Schema Migrator",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.New()

			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.AutomaticEnv()

			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				configName := f.Name
				if !f.Changed && v.IsSet(configName) {
					val := v.Get(configName)
					err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
					if err != nil {
						panic(err)
					}
				}
			})
		},
	}

	var dsn string
	var replicationEnabled bool
	var clusterName string
	var downMigrations []string

	cmd.PersistentFlags().StringVar(&dsn, "dsn", "", "Clickhouse DSN")
	cmd.PersistentFlags().BoolVar(&replicationEnabled, "replication", false, "Enable replication")
	cmd.PersistentFlags().StringVar(&clusterName, "cluster-name", "cluster", "Cluster name to use while running migrations")
	cmd.PersistentFlags().StringArrayVar(&downMigrations, "down", []string{}, "Down migrations to run")

	registerSyncMigrate(cmd)
	registerAsyncMigrate(cmd)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func registerSyncMigrate(cmd *cobra.Command) {

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Run migrations in sync mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()

			opts, err := clickhouse.ParseDSN(dsn)
			if err != nil {
				return fmt.Errorf("failed to parse dsn: %w", err)
			}

			conn, err := clickhouse.Open(opts)
			if err != nil {
				return fmt.Errorf("failed to open connection: %w", err)
			}

			manager, err := schema_migrator.NewMigrationManager(
				schema_migrator.WithClusterName(clusterName),
				schema_migrator.WithReplicationEnabled(replicationEnabled),
				schema_migrator.WithConn(conn),
				schema_migrator.WithLogger(getLogger()),
			)
			if err != nil {
				return fmt.Errorf("failed to create migration manager: %w", err)
			}
			err = manager.Bootstrap()
			if err != nil {
				return fmt.Errorf("failed to bootstrap migrations: %w", err)
			}
			err = manager.RunSquashedMigrations(context.Background())
			if err != nil {
				return fmt.Errorf("failed to run migrations: %w", err)
			}
			return manager.MigrateUpSync(context.Background())
		},
	}

	cmd.AddCommand(syncCmd)
}

func registerAsyncMigrate(cmd *cobra.Command) {

	asyncCmd := &cobra.Command{
		Use:   "async",
		Short: "Run migrations in async mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()

			opts, err := clickhouse.ParseDSN(dsn)
			if err != nil {
				return fmt.Errorf("failed to parse dsn: %w", err)
			}
			conn, err := clickhouse.Open(opts)
			if err != nil {
				return fmt.Errorf("failed to open connection: %w", err)
			}

			manager, err := schema_migrator.NewMigrationManager(
				schema_migrator.WithClusterName(clusterName),
				schema_migrator.WithReplicationEnabled(replicationEnabled),
				schema_migrator.WithConn(conn),
			)
			if err != nil {
				return fmt.Errorf("failed to create migration manager: %w", err)
			}

			return manager.MigrateUpAsync(context.Background())
		},
	}

	cmd.AddCommand(asyncCmd)
}
