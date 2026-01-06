package signozclickhousemetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
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
		QueueBatchConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
			QueueSize:    100,
			NumConsumers: 1,
			Batch: configoptional.Default(exporterhelper.BatchConfig{
				FlushTimeout: 200 * time.Millisecond,
				Sizer:        exporterhelper.RequestSizerTypeItems,
				MinSize:      8192,
			}),
		}),
	}
	err := cfg.Validate()
	require.NoError(t, err)
}
