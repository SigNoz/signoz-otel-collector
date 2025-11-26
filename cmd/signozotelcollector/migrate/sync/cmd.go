package sync

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func Register(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCommand := &cobra.Command{
		Use:   "sync",
		Short: "Runs 'sync' migrations for the store. Sync migrations are used to mutate schemas of the store. These migrations need to be successfully applied before bringing up the application.",
	}

	registerCheck(syncCommand, logger)
	parentCmd.AddCommand(syncCommand)
}
