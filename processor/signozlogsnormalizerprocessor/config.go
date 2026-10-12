package signozlogsnormalizerprocessor

import "fmt"

type Position string

const (
	PositionAfter  Position = "after"
	PositionBefore Position = "before"
)

type Config struct {
	OpAMP OpAMPConfig `mapstructure:"opamp"`
	Body  BodyConfig  `mapstructure:"body"`
}

type OpAMPConfig struct {
	Position Position `mapstructure:"position"`
}

type BodyConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	MessageFields []string `mapstructure:"message_fields"`
}

func (c *Config) Validate() error {
	switch c.OpAMP.Position {
	case PositionAfter, PositionBefore:
		return nil
	default:
		return fmt.Errorf("invalid opamp.position %q: must be %q or %q", c.OpAMP.Position, PositionAfter, PositionBefore)
	}
}
