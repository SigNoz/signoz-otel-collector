package signozspanmappingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozspanmappingprocessor"

import (
	"fmt"
	"strings"
)

// Context is the attribute namespace a rule reads from or writes to.
type Context string

const (
	ContextAttribute Context = "attribute"
	ContextResource  Context = "resource"
)

func (c Context) String() string { return string(c) }

// Action determines whether a source attribute is kept (copy) or deleted
// after the value is written to the target (move).
type Action string

const (
	ActionCopy Action = "copy"
	ActionMove Action = "move"
)

func (a Action) String() string { return string(a) }

const resourcePrefix = "resource."

// Config is the top-level processor configuration.
type Config struct {
	Groups []Group `mapstructure:"groups"`
}

type Group struct {
	ID         string          `mapstructure:"id"`
	ExistsAny  ExistsAny       `mapstructure:"exists_any"`
	Attributes []AttributeRule `mapstructure:"attributes"`
}

// ExistsAny describes when a group's rules should run. A group is considered
// to match when any attribute or resource key on the span CONTAINS one of the
// configured substrings (no glob syntax — plain substring match).
type ExistsAny struct {
	// Attributes holds substrings matched against span/log attribute keys.
	Attributes []string `mapstructure:"attributes"`
	// Resource holds substrings matched against resource attribute keys.
	Resource []string `mapstructure:"resource"`
}

// AttributeRule describes how to populate a single target attribute.
type AttributeRule struct {
	// Target is the attribute key to write.
	Target  string   `mapstructure:"target"`
	Context Context  `mapstructure:"context"`
	Sources []string `mapstructure:"sources"`
	Action  Action   `mapstructure:"action"`
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	for i, g := range c.Groups {
		if len(g.ExistsAny.Attributes) == 0 && len(g.ExistsAny.Resource) == 0 {
			return fmt.Errorf("group[%d] (id=%q): exists_any must have at least one attribute or resource substring", i, g.ID)
		}
		for _, p := range g.ExistsAny.Attributes {
			if p == "" {
				return fmt.Errorf("group[%d] (id=%q): exists_any.attributes substring must not be empty", i, g.ID)
			}
		}
		for _, p := range g.ExistsAny.Resource {
			if p == "" {
				return fmt.Errorf("group[%d] (id=%q): exists_any.resource substring must not be empty", i, g.ID)
			}
		}

		for j, rule := range g.Attributes {
			if strings.TrimSpace(rule.Target) == "" {
				return fmt.Errorf("group[%d] (id=%q) attribute[%d]: target must not be empty", i, g.ID, j)
			}
			if len(rule.Sources) == 0 {
				return fmt.Errorf("group[%d] (id=%q) attribute[%d] (target=%q): sources must not be empty", i, g.ID, j, rule.Target)
			}
			ctx := rule.Context
			if ctx == "" {
				ctx = ContextAttribute
			}
			if ctx != ContextAttribute && ctx != ContextResource {
				return fmt.Errorf("group[%d] (id=%q) attribute[%d] (target=%q): unknown context %q, must be %q or %q",
					i, g.ID, j, rule.Target, ctx, ContextAttribute, ContextResource)
			}
			action := rule.Action
			if action == "" {
				action = ActionCopy
			}
			if action != ActionCopy && action != ActionMove {
				return fmt.Errorf("group[%d] (id=%q) attribute[%d] (target=%q): unknown action %q, must be %q or %q",
					i, g.ID, j, rule.Target, action, ActionCopy, ActionMove)
			}
		}
	}
	return nil
}

// isResourceKey reports whether a source key refers to a resource attribute
// and returns the bare key without the "resource." prefix. This is separate
// from the substring match in exists_any — it only applies to mapper source
// keys.
func isResourceKey(key string) (string, bool) {
	bare, found := strings.CutPrefix(key, resourcePrefix)
	return bare, found
}
