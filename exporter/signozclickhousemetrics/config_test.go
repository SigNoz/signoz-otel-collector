package signozclickhousemetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		DSN:                      "tcp://localhost:9000?database=default",
		SeriesCache:              SeriesCacheConfig{MaxCost: seriesCacheMaxCost, NumCounters: seriesCacheNumCounters},
		MetadataWriteSampleRatio: 1.0,
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_MetadataWriteSampleRatio(t *testing.T) {
	base := func() *Config {
		return &Config{
			DSN:         "tcp://localhost:9000?database=default",
			SeriesCache: SeriesCacheConfig{MaxCost: seriesCacheMaxCost, NumCounters: seriesCacheNumCounters},
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
