package signozddstatsprocessor

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Enabled bool `mapstructure:"enabled"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	return nil
}
