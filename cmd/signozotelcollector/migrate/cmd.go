package migrate

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func Register(parentCmd *cobra.Command, logger *zap.Logger) {
	rootCmd := &cobra.Command{
		Use:               "migrate",
		Short:             "Runs migrations for telemetry store.",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	registerReady(rootCmd, logger)
	registerBootstrap(rootCmd, logger)
	registerSync(rootCmd, logger)
	registerAsync(rootCmd, logger)

	parentCmd.AddCommand(rootCmd)
}
