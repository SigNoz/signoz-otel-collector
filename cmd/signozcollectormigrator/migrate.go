package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/SigNoz/signoz-otel-collector/migrator"
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
	zap.ReplaceGlobals(logger)
}

func main() {
	logger := zap.L().With(zap.String("component", "migrate cli"))
	f := pflag.NewFlagSet("Collector Migrator CLI Options", pflag.ExitOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	f.String("dsn", "", "Clickhouse DSN")
	f.String("cluster-name", "", "Cluster name to use while running migrations")
	f.Bool("multi-node-cluster", false, "True if the dsn points to a multi node clickhouse cluster, false otherwise. Defaults to false.")

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

	multiNodeCluster, err := f.GetBool("multi-node-cluster")
	if err != nil {
		logger.Fatal("Failed to get multi node cluster flag from args", zap.Error(err))
	}

	if dsn == "" || clusterName == "" {
		logger.Fatal("dsn and clusterName are required fields")
	}

	migrationManager, err := migrator.NewMigrationManager(dsn, clusterName, multiNodeCluster)
	if err != nil {
		logger.Fatal("Failed to create migration manager", zap.Error(err))
	}
	defer migrationManager.Close()

	err = migrationManager.Migrate(context.Background())
	if err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}
}
