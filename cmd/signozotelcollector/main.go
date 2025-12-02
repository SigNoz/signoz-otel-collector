package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate"
	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/service"
	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger, err := initZapLog()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}

	rootCmd := &cobra.Command{
		Use: "signoz-otel-collector",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.New()

			v.SetEnvPrefix("signoz-otel-collector")
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.AutomaticEnv()

			cmd.Flags().VisitAll(func(f *flag.Flag) {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			collectorConfig := config.Collector.Config
			managerConfig := config.Collector.ManagerConfig
			copyPath := config.Collector.CopyPath
			if managerConfig != "" {
				if err := copyConfigFile(collectorConfig, copyPath); err != nil {
					logger.Fatal("Failed to copy config file %v", zap.Error(err))
				}
				collectorConfig = copyPath
			}

			ctx := context.Background()

			coll := signozcol.New(
				signozcol.WrappedCollectorSettings{
					ConfigPaths:  []string{collectorConfig},
					Version:      constants.Version,
					Desc:         constants.Desc,
					LoggingOpts:  []zap.Option{zap.WithCaller(true)},
					PollInterval: 200 * time.Millisecond,
					Logger:       logger,
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

			return err
		},
	}

	config.Clickhouse.RegisterFlags(rootCmd)
	config.Collector.RegisterFlags(rootCmd)

	migrate.Register(rootCmd, logger)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
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

func copyConfigFile(configPath string, copyPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configPath)
	}

	return copy(configPath, copyPath)
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
