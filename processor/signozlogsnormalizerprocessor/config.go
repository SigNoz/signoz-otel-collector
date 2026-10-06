package signozlogsnormalizerprocessor

type Config struct {
	JSONBodyDualIngestion bool     `mapstructure:"json_body_dual_ingestion"`
	MessageFields         []string `mapstructure:"message_fields"`
}
