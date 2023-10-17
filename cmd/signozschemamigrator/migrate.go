package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/SigNoz/signoz-otel-collector/migrationmanager"
	"github.com/spf13/pflag"
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
	logger := zap.L().With(zap.String("component", "migrate cli"))
	f := pflag.NewFlagSet("Collector Migrator CLI Options", pflag.ExitOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(1)
	}

	f.String("dsn", "", "Clickhouse DSN")
	f.String("cluster-name", "cluster", "Cluster name to use while running migrations")
	f.Bool("disable-duration-sort-feature", false, "Flag to disable the duration sort feature. Defaults to false.")
	f.Bool("disable-timestamp-sort-feature", false, "Flag to disable the timestamp sort feature. Defaults to false.")

	err := f.Parse(os.Args[1:])
	if err != nil {
		logger.Fatal("Failed to parse args", zap.Error(err))
	}

	dsn, err := f.GetString("dsn")
	if err != nil {
		logger.Fatal("Failed to get dsn from args", zap.Error(err))
	}

	clusterName, err := f.GetString("cluster-name")
	if err != nil {
		logger.Fatal("Failed to get cluster name from args", zap.Error(err))
	}

	disableDurationSortFeature, err := f.GetBool("disable-duration-sort-feature")
	if err != nil {
		logger.Fatal("Failed to get disable duration sort feature flag from args", zap.Error(err))
	}

	disableTimestampSortFeature, err := f.GetBool("disable-timestamp-sort-feature")
	if err != nil {
		logger.Fatal("Failed to get disable timestamp sort feature flag from args", zap.Error(err))
	}

	if dsn == "" {
		logger.Fatal("dsn is a required field")
	}

	// set cluster env so that golang-migrate can use it
	// the value of this env would replace all occurences of {{.SIGNOZ_CLUSTER}} in the migration files
	os.Setenv("SIGNOZ_CLUSTER", clusterName)

	manager, err := migrationmanager.New(dsn, clusterName, disableDurationSortFeature, disableTimestampSortFeature)
	if err != nil {
		logger.Fatal("Failed to create migration manager", zap.Error(err))
	}
	defer manager.Close()

	err = manager.Migrate(context.Background())
	if err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}
}
