// Stanza operators registry dedicated to Signoz logs pipelines

package signozlogspipelinestanzaoperator

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

var SignozStanzaOperatorsRegistry = operator.NewRegistry()

// Register will register an operator in the default registry
func Register(operatorType string, newBuilder func() operator.Builder) {
	SignozStanzaOperatorsRegistry.Register(operatorType, newBuilder)
}

// Lookup looks up a given operator type.Its second return value will
// be false if no builder is registered for that type.
func Lookup(configType string) (func() operator.Builder, bool) {
	return SignozStanzaOperatorsRegistry.Lookup(configType)
}
