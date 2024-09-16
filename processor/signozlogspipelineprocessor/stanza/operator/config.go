// Mostly Brought in as is from opentelemetry-collector-contrib
// Maintaining our own copy/version of Config allows us to use our own
// registry of stanza operators used in Config.Unmarshal in this file

package signozlogspipelinestanzaoperator

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/confmap"
)

// Config is the configuration of an operator
type Config struct {
	operator.Builder
}

// NewConfig wraps the builder interface in a concrete struct
func NewConfig(b operator.Builder) Config {
	return Config{Builder: b}
}

func (c *Config) Unmarshal(component *confmap.Conf) error {
	if !component.IsSet("type") {
		return fmt.Errorf("missing required field 'type'")
	}

	typeInterface := component.Get("type")

	typeString, ok := typeInterface.(string)
	if !ok {
		return fmt.Errorf("non-string type %T for field 'type'", typeInterface)
	}

	builderFunc, ok := SignozStanzaOperatorsRegistry.Lookup(typeString)
	if !ok {
		return fmt.Errorf("unsupported type '%s'", typeString)
	}

	builder := builderFunc()
	if err := component.Unmarshal(builder, confmap.WithIgnoreUnused()); err != nil {
		return fmt.Errorf("unmarshal to %s: %w", typeString, err)
	}

	c.Builder = builder
	return nil
}
