package signozlogsnormalizerprocessor

type Config struct {
	Body   BodyConfig   `mapstructure:"body"`
	Fields FieldsConfig `mapstructure:"fields"`
}

type BodyConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	MessageFields []string `mapstructure:"message_fields"`
}

type FieldsConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	SeverityNumber []string `mapstructure:"severity_number"`
	SeverityText   []string `mapstructure:"severity_text"`
	TraceID        []string `mapstructure:"trace_id"`
	SpanID         []string `mapstructure:"span_id"`
	ScopeName      []string `mapstructure:"scope_name"`
	ScopeVersion   []string `mapstructure:"scope_version"`
}
