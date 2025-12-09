package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	schema_migrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/SigNoz/signoz-otel-collector/constants"
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
	var development bool

	cmd.PersistentFlags().StringVar(&dsn, "dsn", "", "Clickhouse DSN")
	cmd.PersistentFlags().BoolVar(&replicationEnabled, "replication", false, "Enable replication")
	cmd.PersistentFlags().StringVar(&clusterName, "cluster-name", "cluster", "Cluster name to use while running migrations")
	cmd.PersistentFlags().BoolVar(&development, "dev", false, "Development mode")

	registerSyncMigrate(cmd)
	registerAsyncMigrate(cmd)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func registerSyncMigrate(cmd *cobra.Command) {

	var upVersions string
	var downVersions string

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Run migrations in sync mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger()

			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()
			development := strings.ToLower(cmd.Flags().Lookup("dev").Value.String()) == "true"

			logger.Info("Running migrations in sync mode", zap.String("dsn", dsn), zap.Bool("replication", replicationEnabled), zap.String("cluster-name", clusterName), zap.Bool("enable-logs-migrations-v2", constants.EnableLogsMigrationsV2))

			upVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("up").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				upVersions = append(upVersions, v)
			}
			logger.Info("Up migrations", zap.Any("versions", upVersions))

			downVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("down").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				downVersions = append(downVersions, v)
			}
			logger.Info("Down migrations", zap.Any("versions", downVersions))

			if len(upVersions) != 0 && len(downVersions) != 0 {
				return fmt.Errorf("cannot provide both up and down migrations")
			}

			opts, err := clickhouse.ParseDSN(dsn)
			if err != nil {
				return fmt.Errorf("failed to parse dsn: %w", err)
			}
			logger.Info("Parsed DSN", zap.Any("opts", opts))

			conn, err := clickhouse.Open(opts)
			if err != nil {
				return fmt.Errorf("failed to open connection: %w", err)
			}
			logger.Info("Opened connection")

			manager, err := schema_migrator.NewMigrationManager(
				schema_migrator.WithClusterName(clusterName),
				schema_migrator.WithReplicationEnabled(replicationEnabled),
				schema_migrator.WithConn(conn),
				schema_migrator.WithConnOptions(*opts),
				schema_migrator.WithLogger(logger),
				schema_migrator.WithDevelopment(development),
			)
			if err != nil {
				return fmt.Errorf("failed to create migration manager: %w", err)
			}
			err = manager.Bootstrap()
			if err != nil {
				return fmt.Errorf("failed to bootstrap migrations: %w", err)
			}
			logger.Info("Bootstrapped migrations")

			err = manager.RunSquashedMigrations(context.Background())
			if err != nil {
				return fmt.Errorf("failed to run squashed migrations: %w", err)
			}
			logger.Info("Ran squashed migrations")

			if len(downVersions) != 0 {
				logger.Info("Migrating down")
				return manager.MigrateDownSync(context.Background(), downVersions)
			}
			logger.Info("Migrating up")
			return manager.MigrateUpSync(context.Background(), upVersions)
		},
	}

	syncCmd.Flags().StringVar(&upVersions, "up", "", "Up migrations to run, comma separated. Leave empty to run all up migrations")
	syncCmd.Flags().StringVar(&downVersions, "down", "", "Down migrations to run, comma separated. Must provide down migrations explicitly to run")

	cmd.AddCommand(syncCmd)
}

func registerAsyncMigrate(cmd *cobra.Command) {

	var upVersions string
	var downVersions string

	asyncCmd := &cobra.Command{
		Use:   "async",
		Short: "Run migrations in async mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger()

			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()
			development := strings.ToLower(cmd.Flags().Lookup("dev").Value.String()) == "true"

			logger.Info("Running migrations in async mode", zap.String("dsn", dsn), zap.Bool("replication", replicationEnabled), zap.String("cluster-name", clusterName), zap.Bool("enable-logs-migrations-v2", constants.EnableLogsMigrationsV2))

			upVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("up").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				upVersions = append(upVersions, v)
			}
			logger.Info("Up migrations", zap.Any("versions", upVersions))

			downVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("down").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				downVersions = append(downVersions, v)
			}
			logger.Info("Down migrations", zap.Any("versions", downVersions))

			if len(upVersions) != 0 && len(downVersions) != 0 {
				return fmt.Errorf("cannot provide both up and down migrations")
			}

			opts, err := clickhouse.ParseDSN(dsn)
			if err != nil {
				return fmt.Errorf("failed to parse dsn: %w", err)
			}
			logger.Info("Parsed DSN", zap.Any("opts", opts))

			conn, err := clickhouse.Open(opts)
			if err != nil {
				return fmt.Errorf("failed to open connection: %w", err)
			}
			logger.Info("Opened connection")

			manager, err := schema_migrator.NewMigrationManager(
				schema_migrator.WithClusterName(clusterName),
				schema_migrator.WithReplicationEnabled(replicationEnabled),
				schema_migrator.WithConn(conn),
				schema_migrator.WithConnOptions(*opts),
				schema_migrator.WithLogger(logger),
				schema_migrator.WithDevelopment(development),
			)
			if err != nil {
				return fmt.Errorf("failed to create migration manager: %w", err)
			}

			if len(downVersions) != 0 {
				logger.Info("Migrating down")
				return manager.MigrateDownAsync(context.Background(), downVersions)
			}
			logger.Info("Migrating up")
			return manager.MigrateUpAsync(context.Background(), upVersions)
		},
	}

	asyncCmd.Flags().StringVar(&upVersions, "up", "", "Up migrations to run, comma separated. Leave empty to run all up migrations")
	asyncCmd.Flags().StringVar(&downVersions, "down", "", "Down migrations to run, comma separated. Must provide down migrations explicitly to run")

	cmd.AddCommand(asyncCmd)
}
