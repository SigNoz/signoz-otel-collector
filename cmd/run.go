package main

import (
	"context"
	"os"
	"time"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/pkg/env"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"github.com/SigNoz/signoz-otel-collector/service"
	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func registerRun(app *cobra.Command) {
	var collectorConfig collectorConfig
	var storageConfig storageConfig

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the otel collector",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			return runCollector(
				cmd.Context(),
				logger,
				storageConfig,
				collectorConfig,
			)
		},
	}

	collectorConfig.registerFlags(cmd)
	storageConfig.registerFlags(cmd)
	app.AddCommand(cmd)
}

func runCollector(
	ctx context.Context,
	logger log.Logger,
	storageConfig storageConfig,
	collectorConfig collectorConfig,
) error {
	// Copy files
	if collectorConfig.managerConfig != "" {
		// Copy the collector config to the copy path ??
		// Why?
		if _, err := os.Stat(collectorConfig.config); os.IsNotExist(err) {
			logger.Errorctx(ctx, "config file does not exist", err)
			return err
		}

		data, err := os.ReadFile(collectorConfig.config)
		if err != nil {
			logger.Errorctx(ctx, "failed to read config file", err)
			return err
		}

		err = os.WriteFile(collectorConfig.copyPath, data, 0600)
		if err != nil {
			logger.Errorctx(ctx, "failed to write config file at copy path", err)
			return err
		}

		// Set the collector config to the new path
		collectorConfig.config = collectorConfig.copyPath
	}

	storage := storage.NewStorage(
		storageConfig.strategy,
		storage.WithHost(storageConfig.host),
		storage.WithPort(storageConfig.port),
		storage.WithUser(storageConfig.user),
		storage.WithPassword(storageConfig.password),
		storage.WithDatabase(storageConfig.database),
	)
	logger.Infoctx(ctx, "initialized storage")

	//Inject the global env here
	env.NewG(env.WithStorage(storage))

	// Assert the zap logger within the logger interface
	zaplogger := logger.(*log.ZapLogger).Getl().Desugar()

	// Create the collector
	collector, err := service.New(
		signozcol.New(
			signozcol.WrappedCollectorSettings{
				ConfigPaths:  []string{collectorConfig.config},
				Version:      constants.Version,
				Desc:         constants.Desc,
				LoggingOpts:  []zap.Option{zap.WithCaller(true)},
				PollInterval: 200 * time.Millisecond,
				Logger:       zaplogger,
			},
		),
		zaplogger,
		collectorConfig.managerConfig,
		collectorConfig.config,
	)
	if err != nil {
		logger.Errorctx(ctx, "failed to create the collector", err)
		return err
	}

	if err := collector.Start(ctx); err != nil {
		logger.Errorctx(ctx, "failed to start the collector", err)
		return err
	}

	// TODO: Move this cancel logic inside collector.Shutdown()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	wait(ctx, logger, collector.Error())
	if err := collector.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
