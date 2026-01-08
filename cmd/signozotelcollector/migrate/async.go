package migrate

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func registerAsync(parentCmd *cobra.Command, logger *zap.Logger) {
	asyncCommand := &cobra.Command{
		Use:   "async",
		Short: "Runs 'async' migrations for the store. Async migrations are used to apply new migrations to the store asynchronously. These include data migrations or mutations which can be applied in the background without blocking the application.",
	}

	registerAsyncCheck(asyncCommand, logger)
	registerAsyncUp(asyncCommand, logger)
	parentCmd.AddCommand(asyncCommand)
}
