package signozlogsinferrerprocessor

type Config struct {
	SeverityNumberFields []string `mapstructure:"severity_number_fields"`
	SeverityTextFields   []string `mapstructure:"severity_text_fields"`
	TraceIDFields        []string `mapstructure:"trace_id_fields"`
	SpanIDFields         []string `mapstructure:"span_id_fields"`
	ScopeNameFields      []string `mapstructure:"scope_name_fields"`
	ScopeVersionFields   []string `mapstructure:"scope_version_fields"`
}
