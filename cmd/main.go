package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SigNoz/signoz-otel-collector/pkg/cfg"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/spf13/cobra"
)

func main() {
	app := &cobra.Command{
		Use:          "signoz-otel-collector",
		Short:        "Signoz OTEL Collector",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cfg.SetCfg(cmd, "signoz_otel_collector")
		},
	}

	// Set a list of common flags across all sub commands
	var logLevel string
	app.PersistentFlags().StringVar(&logLevel, "log-level", "debug", "The log level of collector. Valid values are [info debug error warn]")

	// register a list of subcommands
	registerMigrate(app)
	registerCollector(app)
	registerApi(app)
	registerRun(app)

	if err := app.Execute(); err != nil {
		os.Exit(1)
	}
}

func wait(ctx context.Context, logger log.Logger, err <-chan error) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		logger.Infoctx(ctx, fmt.Sprintf("caught context error %s, exiting", ctx.Err().Error()))
	case s := <-interrupt:
		logger.Infoctx(ctx, fmt.Sprintf("caught signal %s, exiting", s.String()))
	case e := <-err:
		logger.Infoctx(ctx, fmt.Sprintf("caught process error %s, exiting", e.Error()))
	}

}
