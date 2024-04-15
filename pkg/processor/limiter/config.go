package limiter

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// The policy to use
	Policy string `mapstructure:"policy"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Policy == PolicyPostgres || cfg.Policy == PolicyRedis {
		return nil
	}

	return errors.Newf(errors.TypeUnsupported, "policy %s is not supported", cfg.Policy)
}
