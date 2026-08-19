// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/metric"

	signozlogspipelinestanzaoperator "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/bytedance/sonic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const operatorType = "normalize"

func init() {
	signozlogspipelinestanzaoperator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new normalize config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: signozstanzahelper.NewTransformerConfig(operatorID, operatorType),

		MessageFields: []string{"message", "body", "log", "msg"},
	}
}

// Config is the configuration of a JSON parser operator.
type Config struct {
	signozstanzahelper.TransformerConfig `mapstructure:",squash"`

	MoveAllFields bool `mapstructure:"move_all_fields"`

	MoveSeverityNumberField bool `mapstructure:"move_severity_number_field"`
	MoveSeverityTextField   bool `mapstructure:"move_severity_text_field"`
	MoveTraceIDField        bool `mapstructure:"move_trace_id_field"`
	MoveSpanIDField         bool `mapstructure:"move_span_id_field"`
	MoveTraceFlagsField     bool `mapstructure:"move_trace_flags_field"`
	MoveScopeNameField      bool `mapstructure:"move_scope_name_field"`
	MoveScopeVersionField   bool `mapstructure:"move_scope_version_field"`

	MessageFields        []string `mapstructure:"message_fields"`
	SeverityNumberFields []string `mapstructure:"severity_number_fields"`
	SeverityTextFields   []string `mapstructure:"severity_text_fields"`
	TraceIDFields        []string `mapstructure:"trace_id_fields"`
	SpanIDFields         []string `mapstructure:"span_id_fields"`
	TraceFlagsFields     []string `mapstructure:"trace_flags_fields"`
	ScopeNameFields      []string `mapstructure:"scope_name_fields"`
	ScopeVersionFields   []string `mapstructure:"scope_version_fields"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	logsProcessed, err := set.MeterProvider.Meter("github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/normalize").Int64Counter(
		"signoz_normalize_operator_logs_processed",
		metric.WithDescription("Number of log entries processed by the normalize operator"),
	)
	if err != nil {
		return nil, err
	}

	return &Processor{
		TransformerOperator: transformerOperator,
		Config:              sonic.Config{UseInt64: true},
		logsProcessed:       logsProcessed,
		fields:              newFieldConfig(c),
	}, nil
}
