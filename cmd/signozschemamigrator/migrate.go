package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/migrationmanager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	// init zap logger
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger %v", err)
	}
	// replace global logger
	// TODO(dhawal1248): move away from global logger
	zap.ReplaceGlobals(logger)
}

func main() {
	app := &cobra.Command{
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
	var verbose bool

	app.PersistentFlags().StringVar(&dsn, "dsn", "", "Clickhouse DSN")
	app.PersistentFlags().BoolVar(&replicationEnabled, "replication", false, "Enable replication")
	app.PersistentFlags().StringVar(&clusterName, "cluster-name", "cluster", "Cluster name to use while running migrations")
	app.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	registerSyncMigrate(app)
	registerAsyncMigrate(app)

	if err := app.Execute(); err != nil {
		os.Exit(1)
	}
}

func registerSyncMigrate(app *cobra.Command) {

	var disableDurationSortFeature bool
	var disableTimestampSortFeature bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run migrations in sync mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose := app.PersistentFlags().Lookup("verbose").Value.String() == "true"
			dsn := app.PersistentFlags().Lookup("dsn").Value.String()
			clusterName := app.PersistentFlags().Lookup("cluster-name").Value.String()
			replicationEnabled := app.PersistentFlags().Lookup("replication").Value.String() == "true"

			logger := zap.L().With(zap.String("component", "migrate cli"))

			if dsn == "" {
				logger.Fatal("dsn is a required field")
			}

			// set cluster env so that golang-migrate can use it
			// the value of this env would replace all occurences of {{.SIGNOZ_CLUSTER}} in the migration files
			// TODO: remove this log after dirtry migration issue is fixed
			logger.Info("Setting env var SIGNOZ_CLUSTER", zap.String("cluster-name", clusterName))
			err := os.Setenv("SIGNOZ_CLUSTER", clusterName)
			if err != nil {
				logger.Fatal("Failed to set env var SIGNOZ_CLUSTER", zap.Error(err))
			}
			// TODO: remove this section after dirtry migration issue is fixed
			clusterNameFromEnv := ""
			for _, kvp := range os.Environ() {
				kvParts := strings.SplitN(kvp, "=", 2)
				if kvParts[0] == "SIGNOZ_CLUSTER" {
					clusterNameFromEnv = kvParts[1]
					break
				}
			}
			if clusterNameFromEnv == "" {
				logger.Fatal("Failed to set env var SIGNOZ_CLUSTER")
			}
			logger.Info("Successfully set env var SIGNOZ_CLUSTER ", zap.String("cluster-name", clusterNameFromEnv))

			// set SIGNOZ_REPLICATED env var so that golang-migrate can use it
			// the value of this env would replace all occurences of {{.SIGNOZ_REPLICATED}} in the migration files
			signozReplicated := ""
			logger.Info("Setting env var SIGNOZ_REPLICATED", zap.Bool("replication", replicationEnabled))
			if replicationEnabled {
				signozReplicated = "Replicated"
			}
			err = os.Setenv("SIGNOZ_REPLICATED", signozReplicated)
			if err != nil {
				logger.Fatal("Failed to set env var SIGNOZ_REPLICATED", zap.Error(err))
			}

			manager, err := migrationmanager.New(
				dsn, clusterName, disableDurationSortFeature, disableTimestampSortFeature, verbose, replicationEnabled)
			if err != nil {
				logger.Fatal("Failed to create migration manager", zap.Error(err))
			}
			defer manager.Close()

			err = manager.Migrate(context.Background())
			if err != nil {
				logger.Fatal("Failed to run migrations", zap.Error(err))
			}
			logger.Info("Migrations completed successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&disableDurationSortFeature, "disable-duration-sort-feature", false, "Flag to disable the duration sort feature. Defaults to false.")
	cmd.Flags().BoolVar(&disableTimestampSortFeature, "disable-timestamp-sort-feature", false, "Flag to disable the timestamp sort feature. Defaults to false.")

	app.AddCommand(cmd)
}

func registerAsyncMigrate(app *cobra.Command) {

	cmd := &cobra.Command{
		Use:   "async",
		Short: "Run migrations in async mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement async migration
			return nil
		},
	}

	app.AddCommand(cmd)
}
