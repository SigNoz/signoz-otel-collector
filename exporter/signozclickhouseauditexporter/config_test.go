package signozclickhouseauditexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
)

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	testCases := []struct {
		name     string
		id       component.ID
		expected component.Config
	}{
		{
			name: "DSNOnly_DefaultsKept",
			id:   component.NewIDWithName(metadata.Type, ""),
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.DSN = "tcp://127.0.0.1:9000"
			}),
		},
		{
			name: "AllFields_Overridden",
			id:   component.NewIDWithName(metadata.Type, "full"),
			expected: withDefaultConfig(func(cfg *Config) {
				cfg.DSN = "tcp://default:password@127.0.0.1:9000?dial_timeout=5s&max_open_conns=30"
				cfg.TimeoutSettings = exporterhelper.TimeoutConfig{Timeout: 10 * time.Second}
				cfg.BackOffConfig.Enabled = true
				cfg.BackOffConfig.InitialInterval = time.Second
				cfg.BackOffConfig.MaxInterval = 10 * time.Second
				cfg.BackOffConfig.MaxElapsedTime = time.Minute
				queue := exporterhelper.NewDefaultQueueConfig()
				queue.QueueSize = 100
				queue.NumConsumers = 4
				cfg.QueueSettings = configoptional.Some(queue)
			}),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig()

			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, testCase.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		name    string
		dsn     string
		wantErr string
	}{
		{name: "MissingDSN", dsn: "", wantErr: "dsn must be specified"},
		{name: "NotURL", dsn: "://localhost:9000", wantErr: "dsn must be url format"},
		{name: "InvalidDSN", dsn: "tcp://localhost:9000?dial_timeout=notaduration", wantErr: "dial timeout"},
		{name: "ValidDSN", dsn: "tcp://default:password@localhost:9000"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := withDefaultConfig(func(cfg *Config) {
				cfg.DSN = testCase.dsn
			})

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

func TestConfig_buildDSN(t *testing.T) {
	testCases := []struct {
		name    string
		dsn     string
		want    string
		wantErr error
	}{
		{
			name: "NoParams_DefaultsAdded",
			dsn:  "tcp://default:password@localhost:9000/signoz_audit",
			want: "tcp://default:password@localhost:9000/signoz_audit?async_insert=true&compress=lz4&input_format_try_infer_dates=0&input_format_try_infer_datetimes=0&type_json_skip_duplicated_paths=1",
		},
		{
			name: "ParamsSet_DSNValuesKept",
			dsn:  "tcp://localhost:9000?async_insert=false&compress=zstd&input_format_try_infer_dates=1",
			want: "tcp://localhost:9000?async_insert=false&compress=zstd&input_format_try_infer_dates=1&input_format_try_infer_datetimes=0&type_json_skip_duplicated_paths=1",
		},
		{
			name:    "NotURL_ReturnsError",
			dsn:     "://localhost:9000",
			wantErr: errConfigInvalidDSN,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := withDefaultConfig(func(cfg *Config) {
				cfg.DSN = testCase.dsn
			})

			got, err := cfg.buildDSN()
			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestConfig_buildClickHouseOptions(t *testing.T) {
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.DSN = "tcp://default:password@localhost:9000"
	})

	options, err := cfg.buildClickHouseOptions()
	require.NoError(t, err)

	assert.Equal(t, clickhouse.Settings{
		"async_insert":                     1,
		"type_json_skip_duplicated_paths":  1,
		"input_format_try_infer_datetimes": 0,
		"input_format_try_infer_dates":     0,
	}, options.Settings)
	assert.Equal(t, clickhouse.CompressionLZ4, options.Compression.Method)
}
