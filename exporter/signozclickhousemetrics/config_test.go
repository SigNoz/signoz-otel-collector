package signozclickhousemetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"

	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
)

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		DSN:                      "tcp://localhost:9000?database=default",
		MetadataWriteSampleRatio: 1.0,
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_MetadataWriteSampleRatio(t *testing.T) {
	base := func() *Config {
		return &Config{
			DSN: "tcp://localhost:9000?database=default",
		}
	}
	for _, valid := range []float64{0.01, 0.5, 1.0} {
		cfg := base()
		cfg.MetadataWriteSampleRatio = valid
		require.NoError(t, cfg.Validate(), "ratio %v should be valid", valid)
	}
	for _, invalid := range []float64{0, -0.1, 1.5} {
		cfg := base()
		cfg.MetadataWriteSampleRatio = invalid
		require.Error(t, cfg.Validate(), "ratio %v should be rejected", invalid)
	}
}

func TestConfig_Validate_TimeBucketedSet(t *testing.T) {
	testCases := []struct {
		name    string
		set     TimeBucketedSetConfig
		wantErr bool
	}{
		{name: "Disabled_InvalidValues_Ignored", set: TimeBucketedSetConfig{Config: timebucketedset.Config{MaxBuckets: 1, PreWriteWindow: 2 * time.Hour}}},
		{name: "Enabled_ZeroValues_TakeDefaults", set: TimeBucketedSetConfig{Enabled: true}},
		{name: "Enabled_PreWriteWindowNotUnderBucket_Rejected", set: TimeBucketedSetConfig{Enabled: true, Config: timebucketedset.Config{PreWriteWindow: time.Hour}}, wantErr: true},
		{name: "Enabled_SingleBucket_Rejected", set: TimeBucketedSetConfig{Enabled: true, Config: timebucketedset.Config{MaxBuckets: 1}}, wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &Config{
				DSN:                      "tcp://localhost:9000?database=default",
				MetadataWriteSampleRatio: 1.0,
				TimeBucketedSet:          testCase.set,
			}
			err := cfg.Validate()
			if testCase.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConfig_Unmarshal_TimeBucketedSet(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	conf := confmap.NewFromStringMap(map[string]any{
		"dsn": "tcp://localhost:9000?database=default",
		"time_bucketed_set": map[string]any{
			"enabled":          true,
			"max_buckets":      4,
			"max_bucket_size":  64 << 20,
			"pre_write_window": "10m",
		},
	})
	require.NoError(t, conf.Unmarshal(cfg))
	require.NoError(t, cfg.(*Config).Validate())
	require.Equal(t, TimeBucketedSetConfig{
		Enabled: true,
		Config:  timebucketedset.Config{MaxBuckets: 4, MaxBucketSize: 64 << 20, PreWriteWindow: 10 * time.Minute},
	}, cfg.(*Config).TimeBucketedSet)
}
