// Brought in as is from logstransform processor in opentelemetry-collector-contrib
package signozlogspipelineprocessor

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type Config struct {
	adapter.BaseConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if len(cfg.BaseConfig.Operators) == 0 {
		return errors.New("no operators were configured for signozlogspipeline processor")
	}
	return nil
}
