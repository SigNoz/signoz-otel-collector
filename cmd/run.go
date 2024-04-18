package main

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/cache"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func registerRun(app *cobra.Command) {
	var adminHttpConfig adminHttpConfig
	var collectorConfig collectorConfig
	var storageConfig storageConfig
	var cacheConfig cacheConfig
	var adminEnabled bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Starts the otel collector and the api",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			// Initialize the storage layer
			storage := storage.NewStorage(
				storageConfig.strategy,
				storage.WithHost(storageConfig.host),
				storage.WithPort(storageConfig.port),
				storage.WithUser(storageConfig.user),
				storage.WithPassword(storageConfig.password),
				storage.WithDatabase(storageConfig.database),
			)
			logger.Infoctx(ctx, "initialized storage", "strategy", storageConfig.strategy)

			// Initialize the cache layer
			cache := cache.NewCache(
				cacheConfig.strategy,
				cache.WithHost(cacheConfig.host),
				cache.WithPort(cacheConfig.port),
			)
			logger.Infoctx(ctx, "initialized cache", "strategy", cacheConfig.strategy)

			return run(
				cmd.Context(),
				logger,
				storage,
				cache,
				collectorConfig,
				adminEnabled,
				adminHttpConfig,
			)
		},
	}

	cmd.Flags().BoolVar(&adminEnabled, "admin-enabled", false, "Whether to enable the admin api or not, do not specify the flag to disable the admin api")
	adminHttpConfig.registerFlags(cmd)
	collectorConfig.registerFlags(cmd)
	storageConfig.registerFlags(cmd)
	cacheConfig.registerFlags(cmd)
	app.AddCommand(cmd)
}

func run(
	ctx context.Context,
	logger log.Logger,
	storage *storage.Storage,
	cache *cache.Cache,
	collectorConfig collectorConfig,
	adminEnabled bool,
	adminHttpConfig adminHttpConfig,
) error {
	group, ctx := errgroup.WithContext(ctx)

	// Start the collector first
	group.Go(func() error {
		return runCollector(ctx, logger, storage, collectorConfig, cache)
	})
	// Start the admin api
	if adminEnabled {
		group.Go(func() error {
			return runApi(ctx, logger, storage, adminHttpConfig)
		})
	}

	err := group.Wait()
	if err != nil {
		return err
	}

	return nil
}
