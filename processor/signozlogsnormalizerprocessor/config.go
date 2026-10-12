package signozlogsnormalizerprocessor

import "fmt"

type UserPipelines string

const (
	UserPipelinesAfter  UserPipelines = "after"
	UserPipelinesBefore UserPipelines = "before"
)

type Config struct {
	UserPipelines UserPipelines `mapstructure:"user_pipelines"`
	Body          BodyConfig    `mapstructure:"body"`
}

type BodyConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	MessageFields []string `mapstructure:"message_fields"`
}

func (c *Config) Validate() error {
	switch c.UserPipelines {
	case UserPipelinesAfter, UserPipelinesBefore:
		return nil
	default:
		return fmt.Errorf("invalid user_pipelines %q: must be %q or %q", c.UserPipelines, UserPipelinesAfter, UserPipelinesBefore)
	}
}
