package signozlogsnormalizerprocessor

import "fmt"

type Position string

const (
	PositionAfter  Position = "after"
	PositionBefore Position = "before"
)

type Config struct {
	OpAMP          OpAMPConfig `mapstructure:"opamp"`
	Body           BodyConfig  `mapstructure:"body"`
	SeverityNumber FieldConfig `mapstructure:"severity_number"`
	SeverityText   FieldConfig `mapstructure:"severity_text"`
	TraceID        FieldConfig `mapstructure:"trace_id"`
	SpanID         FieldConfig `mapstructure:"span_id"`
	ScopeName      FieldConfig `mapstructure:"scope_name"`
	ScopeVersion   FieldConfig `mapstructure:"scope_version"`
}

type OpAMPConfig struct {
	Position Position `mapstructure:"position"`
}

type BodyConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Fields  []string `mapstructure:"fields"`
}

type FieldConfig struct {
	Fields []string `mapstructure:"fields"`
	Move   bool     `mapstructure:"move"`
}

func (c *Config) Validate() error {
	switch c.OpAMP.Position {
	case PositionAfter, PositionBefore:
		return nil
	default:
		return fmt.Errorf("invalid opamp.position %q: must be %q or %q", c.OpAMP.Position, PositionAfter, PositionBefore)
	}
}
