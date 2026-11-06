package signozmetricreductionrulesextension

import (
	"errors"
	"time"
)

// Config defines configuration for the metric reduction rules extension.
type Config struct {
	DSN string `mapstructure:"dsn"`

	Database string `mapstructure:"database"`

	// PollInterval is how often the rules table is re-read. effective_from is set
	// ahead by the writer so poll cadence doesn't affect correctness within that margin.
	PollInterval time.Duration `mapstructure:"poll_interval"`
	RulesTable   string        `mapstructure:"rules_table"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if cfg.PollInterval < 5*time.Second {
		return errors.New("poll_interval must be at least 5s")
	}
	if cfg.RulesTable == "" {
		return errors.New("rules_table must be specified")
	}
	return nil
}
