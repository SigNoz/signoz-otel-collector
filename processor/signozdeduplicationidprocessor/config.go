package signozdeduplicationidprocessor

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Generator string `mapstructure:"generator"`
}

var _ component.Config = (*Config)(nil)

func (config *Config) Validate() error {
	return nil
}
