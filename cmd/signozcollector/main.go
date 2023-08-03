package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/service"
	"github.com/SigNoz/signoz-otel-collector/signozcol"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	logger, err := initZapLog()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}

	collectorConfig, _ := f.GetString("config")
	if err := copyConfigFile(collectorConfig); err != nil {
		logger.Fatal("Failed to copy config file %v", zap.Error(err))
	}
	managerConfig, _ := f.GetString("manager-config")
	fmt.Println("managerConfig", managerConfig)

	ctx := context.Background()

	coll := signozcol.New(
		signozcol.WrappedCollectorSettings{
			ConfigPaths:  []string{constants.CopyPath},
			Version:      constants.Version,
			Desc:         constants.Desc,
			LoggingOpts:  []zap.Option{zap.WithCaller(true)},
			PollInterval: 200 * time.Millisecond,
		},
	)

	svc, err := service.New(coll, logger, managerConfig, collectorConfig)
	if err != nil {
		logger.Fatal("failed to create collector service:", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := runInteractive(ctx, logger, svc); err != nil {
		logger.Fatal("failed to run service:", zap.Error(err))
	}
}

func runInteractive(ctx context.Context, logger *zap.Logger, svc service.Service) error {
	if err := svc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start collector service: %w", err)
	}

	// Wait for context done or service error
	select {
	case <-ctx.Done():
		logger.Info("Context done, shutting down...")
	case err := <-svc.Error():
		logger.Error("Service error, shutting down...", zap.Error(err))
	}

	stopTimeoutCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := svc.Shutdown(stopTimeoutCtx); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

func initZapLog() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := config.Build()
	return logger, err
}

func copyConfigFile(configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configPath)
	}

	copy(configPath, constants.CopyPath)
	return nil
}

func copy(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", src, err)
	}

	err = os.WriteFile(dest, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write to dest file %s: %w", dest, err)
	}

	return nil
}
