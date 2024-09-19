// Brought in as is from logstransform processor in opentelemetry-collector-contrib
package signozlogspipelineprocessor

import (
	"errors"

	signozlogspipelinestanzaadapter "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	signozlogspipelinestanzaadapter.BaseConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if len(cfg.BaseConfig.Operators) == 0 {
		return errors.New("no operators were configured for signozlogspipeline processor")
	}
	return nil
}

func (cfg *Config) OperatorConfigs() []operator.Config {
	ops := []operator.Config{}

	for _, op := range cfg.BaseConfig.Operators {
		ops = append(ops, operator.Config(op))
	}
	return ops
}
