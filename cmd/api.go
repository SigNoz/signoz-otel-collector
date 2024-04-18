package main

import (
	"context"

	httpAPI "github.com/SigNoz/signoz-otel-collector/pkg/api/http"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/server/http"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"github.com/spf13/cobra"
)

func registerApi(app *cobra.Command) {
	var adminHttpConfig adminHttpConfig
	var storageConfig storageConfig

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Starts an api to interact with configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			storage := storage.NewStorage(
				storageConfig.strategy,
				storage.WithHost(storageConfig.host),
				storage.WithPort(storageConfig.port),
				storage.WithUser(storageConfig.user),
				storage.WithPassword(storageConfig.password),
				storage.WithDatabase(storageConfig.database),
			)
			logger.Infoctx(ctx, "initialized storage", "strategy", storageConfig.strategy)

			return runApi(
				cmd.Context(),
				logger,
				storage,
				adminHttpConfig,
			)
		},
	}

	adminHttpConfig.registerFlags(cmd)
	storageConfig.registerFlags(cmd)
	app.AddCommand(cmd)
}

func runApi(
	ctx context.Context,
	logger log.Logger,
	storage *storage.Storage,
	adminHttpConfig adminHttpConfig,
) error {
	// Create the api and register all apis into the base api
	api := httpAPI.NewAPI(logger)
	httpAPI.NewTenantAPI(api, storage).Register()
	httpAPI.NewKeyAPI(api, storage).Register()

	// Create the server
	server := http.NewServer(logger, api.Handler(), http.WithListen(adminHttpConfig.bindAddress))

	// Run the http server in a goroutine and catch potential errors
	ch := make(chan error, 1)
	go func() {
		if err := server.Start(ctx); err != nil {
			ch <- err
		}
	}()

	wait(ctx, logger, ch)
	if err := server.Stop(ctx); err != nil {
		return err
	}
	return nil
}
