package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SigNoz/signoz-otel-collector/collector"
	"github.com/SigNoz/signoz-otel-collector/service"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
)

func main() {

	// Command line flags
	f := flag.NewFlagSet("Collector CLI Options", flag.ExitOnError)

	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	f.String("config", "", "File path for the collector configuration")
	f.String("manager-config", "", "File path for the agent manager configuration")
	err := f.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse args %v", err)
	}

	collectorConfig, _ := f.GetString("config")
	managerConfig, _ := f.GetString("manager-config")

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}

	ctx := context.Background()

	coll := collector.New(
		collector.WrappedCollectorSettings{
			ConfigPaths: []string{collectorConfig},
			// TODO: Build version from git tag
			Version:     "0.63.0",
			Desc:        "SigNoz OpenTelemetry Collector",
			LoggingOpts: []zap.Option{zap.WithCaller(true)},
		},
	)

	svc, err := service.New(coll, logger, managerConfig, collectorConfig)
	if err != nil {
		log.Fatalf("failed to create collector service: %v", err)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := runInteractive(ctx, logger, svc); err != nil {
		log.Fatalf("failed to run service: %v", err)
	}
}

func runInteractive(ctx context.Context, logger *zap.Logger, svc service.Service) error {
	if err := svc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start collector service: %w", err)
	}

	select {
	case <-ctx.Done():
	}

	stopTimeoutCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := svc.Shutdown(stopTimeoutCtx); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}
