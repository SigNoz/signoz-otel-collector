package signozclickhouseauditexporter

import (
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		name    string
		dsn     string
		wantErr string
	}{
		{name: "MissingDSN", dsn: "", wantErr: "dsn must be specified"},
		{name: "InvalidDSN", dsn: "tcp://localhost:9000?dial_timeout=notaduration", wantErr: "invalid dsn"},
		{name: "ValidDSN", dsn: "tcp://default:password@localhost:9000"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.DSN = testCase.dsn

			err := cfg.Validate()
			if testCase.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.wantErr)
		})
	}
}

func TestConfigClickHouseOptions(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.DSN = "tcp://default:password@localhost:9000"

	options, err := cfg.clickHouseOptions()
	require.NoError(t, err)

	assert.Equal(t, clickhouse.Settings{"type_json_skip_duplicated_paths": 1}, options.Settings)
	assert.Equal(t, cfg.QueueBatchConfig.Get().NumConsumers+1, options.MaxIdleConns)
}
