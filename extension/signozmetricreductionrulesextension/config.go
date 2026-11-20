package signozmetricreductionrulesextension

import (
	"errors"
	"time"
)

type Config struct {
	DSN string `mapstructure:"dsn"`

	// PollInterval is how often the rules table is re-read. effective_from is set
	// ahead by the writer so poll cadence doesn't affect correctness within that margin.
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if cfg.PollInterval < 5*time.Second {
		return errors.New("poll_interval must be at least 5s")
	}
	return nil
}
