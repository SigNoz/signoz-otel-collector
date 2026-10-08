package signozlogsnormalizerprocessor

type Config struct {
	Body BodyConfig `mapstructure:"body"`
}

type BodyConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	MessageFields []string `mapstructure:"message_fields"`
}
