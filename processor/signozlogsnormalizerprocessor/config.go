package signozlogsnormalizerprocessor

import "fmt"

type Position string

const (
	PositionFirst Position = "first"
	PositionLast  Position = "last"
)

type Config struct {
	Position Position   `mapstructure:"position"`
	Body     BodyConfig `mapstructure:"body"`
}

type BodyConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	MessageFields []string `mapstructure:"message_fields"`
}

func (c *Config) Validate() error {
	switch c.Position {
	case PositionFirst, PositionLast:
		return nil
	default:
		return fmt.Errorf("invalid position %q: must be %q or %q", c.Position, PositionFirst, PositionLast)
	}
}
