package signozclickhousemeter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		DSN: "tcp://localhost:9000?database=default",
		QueueBatchConfig: exporterhelper.QueueBatchConfig{
			QueueSize:    100,
			NumConsumers: 1,
		},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}
