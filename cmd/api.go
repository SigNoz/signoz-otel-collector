package main

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/server/http"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func registerApi(app *cobra.Command) {
	var httpAddress string

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Starts an api to interact with configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()
			server := http.NewServer(logger, gin.Default(), http.WithListen(httpAddress))

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
