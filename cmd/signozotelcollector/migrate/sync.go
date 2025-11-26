package migrate

import (
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/sync"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func registerSync(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCommand := &cobra.Command{
		Use:   "sync",
		Short: "Runs 'sync' migrations for the store. Sync migrations are used to mutate schemas of the store. These migrations need to be successfully applied before bringing up the application.",
	}

	sync.RegisterCheck(syncCommand, logger)
	parentCmd.AddCommand(syncCommand)
}
