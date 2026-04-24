package signozspanmappingprocessor // import "github.com/SigNoz/signoz-otel-collector/processor/signozspanmappingprocessor"

import (
	"fmt"
	"path"
	"strings"
)

const (
	ActionCopy = "copy"
	ActionMove = "move"

	ContextAttributes = "attributes"
	ContextResource   = "resource"

	// resourcePrefix denotes a source key that reads from resource attributes.
	// e.g. "resource.service.name" → resource attribute "service.name".
	resourcePrefix = "resource."
)

// Config is the top-level processor configuration.
type Config struct {
	Groups []Group `mapstructure:"groups"`
}

// Group is a conditional attribute-mapping block. Rules are applied only when
// the exists_any condition is satisfied.
type Group struct {
	// ID is a human-readable label used in log/error messages.
	ID string `mapstructure:"id"`

	// ExistsAny holds glob patterns checked against span and/or resource
	// attribute keys. The group is applied when at least one key in the span
	// or resource matches any pattern. Short-circuits on first match.
	ExistsAny ExistsAny `mapstructure:"exists_any"`

	// Attributes is the ordered list of mapping rules executed when the group
	// condition is met.
	Attributes []AttributeRule `mapstructure:"attributes"`
}

// ExistsAny separates span-attribute patterns from resource-attribute patterns.
type ExistsAny struct {
	// Attributes holds glob patterns matched against span/log attribute keys.
	Attributes []string `mapstructure:"attributes"`

	// Resource holds glob patterns matched against resource attribute keys.
	Resource []string `mapstructure:"resource"`
}

// AttributeRule describes how to populate a single target attribute.
type AttributeRule struct {
	// Target is the attribute key to write.
	Target string `mapstructure:"target"`

	// Context controls where Target is written.
	//   "attributes" (default) — span / log attributes.
	//   "resource"             — resource attributes.
	Context string `mapstructure:"context"`

	// Sources is an ordered list of keys to read. The first key that exists
	// wins. Keys prefixed with "resource." are looked up in resource
	// attributes; all others in span/log attributes.
	Sources []string `mapstructure:"sources"`

	// Action controls what happens to the source key after copy.
	//   "copy" (default) — source key is preserved.
	//   "move"           — source key is deleted after copying.
	Action string `mapstructure:"action"`
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	for i, g := range c.Groups {
		if len(g.ExistsAny.Attributes) == 0 && len(g.ExistsAny.Resource) == 0 {
			return fmt.Errorf("group[%d] (id=%q): exists_any must have at least one attribute or resource pattern", i, g.ID)
		}

		allPatterns := append(g.ExistsAny.Attributes, g.ExistsAny.Resource...)
		for _, p := range allPatterns {
			if _, err := path.Match(p, ""); err != nil {
				return fmt.Errorf("group[%d] (id=%q): invalid glob pattern %q: %w", i, g.ID, p, err)
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
				ctx = ContextAttributes
			}
			if ctx != ContextAttributes && ctx != ContextResource {
				return fmt.Errorf("group[%d] (id=%q) attribute[%d] (target=%q): unknown context %q, must be %q or %q",
					i, g.ID, j, rule.Target, ctx, ContextAttributes, ContextResource)
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
// and returns the bare key without the prefix.
func isResourceKey(key string) (string, bool) {
	bare, found := strings.CutPrefix(key, resourcePrefix)
	return bare, found
}
