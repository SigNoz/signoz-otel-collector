package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run migrations in sync mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement sync migration
			return nil
		},
	}

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
