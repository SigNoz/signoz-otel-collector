package signozmemorylimiterprocessor

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

var (
	errLimitOutOfRange = errors.New("'limit_mib' must be greater than zero")
)

type Config struct {
	// MemoryLimitMiB is the maximum amount of memory, in MiB, targeted to be
	// allocated by the process.
	MemoryLimitMiB uint32 `mapstructure:"limit_mib"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.MemoryLimitMiB == 0 {
		return errLimitOutOfRange
	}

	return nil
}
