package signozlogsnormalizerprocessor

import (
	"errors"
	"fmt"
)

type Position string

const (
	PositionAfter  Position = "after"
	PositionBefore Position = "before"
)

type Action string

const (
	ActionMove Action = "move"
	ActionCopy Action = "copy"
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
	Enabled bool     `mapstructure:"enabled"`
	Fields  []string `mapstructure:"fields"`
	Action  Action   `mapstructure:"action"`
}

func (c *Config) fields() [targetCount]*FieldConfig {
	return [targetCount]*FieldConfig{
		targetSeverityNumber: &c.SeverityNumber,
		targetSeverityText:   &c.SeverityText,
		targetTraceID:        &c.TraceID,
		targetSpanID:         &c.SpanID,
		targetScopeName:      &c.ScopeName,
		targetScopeVersion:   &c.ScopeVersion,
	}
}

func (c *Config) Validate() error {
	var errs []error
	switch c.OpAMP.Position {
	case PositionAfter, PositionBefore:
	default:
		errs = append(errs, fmt.Errorf("invalid opamp.position %q: must be %q or %q", c.OpAMP.Position, PositionAfter, PositionBefore))
	}
	for t, field := range c.fields() {
		switch field.Action {
		case ActionMove, ActionCopy:
		default:
			errs = append(errs, fmt.Errorf("invalid %s.action %q: must be %q or %q", targetNames[t], field.Action, ActionMove, ActionCopy))
		}
	}
	return errors.Join(errs...)
}
