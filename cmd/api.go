package main

import (
	httpAPI "github.com/SigNoz/signoz-otel-collector/pkg/api/http"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/server/http"
	"github.com/spf13/cobra"
)

func registerApi(app *cobra.Command) {
	var httpAddress string

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Starts an api to interact with configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			// Create the api
			api := httpAPI.NewAPI(logger)
			// Register all apis into api
			httpAPI.NewKeyAPI(api).Register()
			httpAPI.NewLimitAPI(api).Register()

			// Create the server
			server := http.NewServer(logger, api.Handler(), http.WithListen(httpAddress))

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

	// Configuration options for the api
	cmd.Flags().StringVar(&httpAddress, "http-address", "0.0.0.0:8000", "Listen ip:port address for all http endpoints.")

	app.AddCommand(cmd)
}
