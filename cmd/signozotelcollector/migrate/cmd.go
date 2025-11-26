package migrate

import (
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/ready"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/sync"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func Register(parentCmd *cobra.Command, logger *zap.Logger) {
	rootCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Runs migrations for any store.",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	sync.Register(rootCmd, logger)
	ready.Register(rootCmd, logger)

	parentCmd.AddCommand(rootCmd)
}
