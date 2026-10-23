package signozfilterprocessor

import "errors"

type Config struct {
	MaxAllowedDataAgeDays int `mapstructure:"max_allowed_data_age_days"`
}

func (c *Config) Validate() error {
	if c.MaxAllowedDataAgeDays <= 0 {
		return errors.New("max_allowed_data_age_days must be positive")
	}
	return nil
}
