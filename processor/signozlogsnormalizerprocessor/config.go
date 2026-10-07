package signozlogsnormalizerprocessor

type Config struct {
	Body BodyConfig `mapstructure:"body"`
}

type BodyConfig struct {
	Enabled               bool     `mapstructure:"enabled"`
	JSONBodyDualIngestion bool     `mapstructure:"json_body_dual_ingestion"`
	MessageFields         []string `mapstructure:"message_fields"`
}
