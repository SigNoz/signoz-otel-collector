package main

import (
	"fmt"
	"os"
	"path"

	"ariga.io/atlas-go-sdk/atlasexec"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage/strategies"
	"github.com/spf13/cobra"
)

func registerMigrate(app *cobra.Command) {
	var storageConfig storageConfig

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database schema migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create the logger by taking the log-level from the root command
			logger := log.NewZapLogger(app.PersistentFlags().Lookup("log-level").Value.String())
			defer func() {
				_ = logger.Flush()
			}()

			//Check the strategy, if the strategy is off, do not run any migrations
			if storageConfig.strategy == strategies.Off {
				logger.Infoctx(cmd.Context(), "no migrations to run for current strategy")
				return nil
			}

			// Initialize atlas
			workdir, err := atlasexec.NewWorkingDir(
				atlasexec.WithAtlasHCLPath(path.Clean("./pkg/storage/migrations/atlas.hcl")),
				atlasexec.WithMigrations(os.DirFS("./pkg/storage/migrations/migrations")),
			)
			if err != nil {
				logger.Errorctx(cmd.Context(), "failed to load working directory", err)
				return err
			}

			// atlasexec works on a temporary directory, so we need to close it
			defer func() {
				err := workdir.Close()
				if err != nil {
					logger.Errorctx(cmd.Context(), "failed to close working directory", err)
				}
			}()

			// Initialize the client.
			client, err := atlasexec.NewClient(workdir.Path(), "atlas")
			if err != nil {
				logger.Errorctx(cmd.Context(), "failed to initialize client", err)
				return err
			}

			// apply the migrations
			res, err := client.MigrateApply(cmd.Context(), &atlasexec.MigrateApplyParams{
				Env:    "default",
				TxMode: "file",
				Vars: atlasexec.Vars{
					"url": fmt.Sprintf(
						"postgres://%s:%s@%s:%d/%s?search_path=public&sslmode=disable",
						storageConfig.user,
						storageConfig.password,
						storageConfig.host,
						storageConfig.port,
						storageConfig.database,
					),
				},
			})
			if err != nil {
				logger.Errorctx(cmd.Context(), "failed to apply migrations", err)
				return err
			}

			logger.Infoctx(cmd.Context(), fmt.Sprintf("applied %d migration(s)", len(res.Applied)))
			for _, applied := range res.Applied {
				logger.Infoctx(cmd.Context(), fmt.Sprintf("applied migration file %s", applied.Name))
			}

			return nil
		},
	}

	storageConfig.registerFlags(cmd)
	app.AddCommand(cmd)
}
