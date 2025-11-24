package migrate

import (
	"fmt"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/config"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/sync"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func Register(parentCmd *cobra.Command, logger *zap.Logger) {
	rootCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Runs migrations for any store.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.New()

			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.AutomaticEnv()

			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				configName := f.Name
				if !f.Changed && v.IsSet(configName) {
					val := v.Get(configName)
					err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
					if err != nil {
						panic(err)
					}
				}
			})
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	config.Clickhouse.RegisterFlags(rootCmd)

	sync.Register(rootCmd, logger)

	parentCmd.AddCommand(rootCmd)
}
