package keyauth

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// The headers containing keyauth
	Headers []string `mapstructure:"headers"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Headers == nil {
		return errors.New("no headers provided")
	}
	return nil
}
