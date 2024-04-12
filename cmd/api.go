package main

import (
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
		Short: "Starts an api to interact with configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			// Initialize the storage layer
			storage := storage.NewStorage(storageConfig.strategy,
				storage.WithHost(storageConfig.host),
				storage.WithPort(storageConfig.port),
				storage.WithUser(storageConfig.user),
				storage.WithPassword(storageConfig.password),
				storage.WithDatabase(storageConfig.database),
			)

			// Create the api and register all apis into the base api
			api := httpAPI.NewAPI(logger)
			httpAPI.NewTenantAPI(api, storage).Register()
			httpAPI.NewKeyAPI(api, storage).Register()

			// Create the server
			server := http.NewServer(logger, api.Handler(), http.WithListen(adminHttpConfig.bindAddress))

			// Run the http server in a goroutine and catch potential errors
			ch := make(chan error, 1)
			go func() {
				if err := server.Start(cmd.Context()); err != nil {
					ch <- err
				}
			}()

			wait(cmd.Context(), logger, ch)
			if err := server.Stop(cmd.Context()); err != nil {
				return err
			}
			return nil
		},
	}

	adminHttpConfig.registerFlags(cmd)
	storageConfig.registerFlags(cmd)
	app.AddCommand(cmd)
}
