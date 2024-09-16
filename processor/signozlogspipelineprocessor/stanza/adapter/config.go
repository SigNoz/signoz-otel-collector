package signozlogspipelinestanzaadapter

import (
	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
)

type BaseConfig struct {
	Operators []signozlogspipelinestanzaoperator.Config `mapstructure:"operators"`
}
