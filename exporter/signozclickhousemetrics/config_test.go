package signozclickhousemetrics

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/pkg/metricreductionrule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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

func TestConfig_Validate_ReductionBufferTables(t *testing.T) {
	rules := &metricreductionrule.Config{Extension: component.MustNewID("signozmetricreductionrules")}
	testCases := []struct {
		name      string
		rules     *metricreductionrule.Config
		reduction ReductionConfig
		err       string
	}{
		{name: "NoRules_EmptyTables_Valid"},
		{name: "Rules_BothTables_Valid", rules: rules, reduction: ReductionConfig{BufferSamplesTable: "samples_buffer", BufferTimeSeriesTable: "ts_buffer"}},
		{name: "Rules_EmptySamplesTable_Error", rules: rules, reduction: ReductionConfig{BufferTimeSeriesTable: "time_series_buffer"}, err: "reduction.buffer_samples_table must be specified"},
		{name: "Rules_EmptyTimeSeriesTable_Error", rules: rules, reduction: ReductionConfig{BufferSamplesTable: "distributed_samples_buffer"}, err: "reduction.buffer_time_series_table must be specified"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &Config{
				DSN:                      "tcp://localhost:9000?database=default",
				MetadataWriteSampleRatio: 1.0,
				Rules:                    testCase.rules,
				Reduction:                testCase.reduction,
			}
			err := cfg.Validate()
			if testCase.err != "" {
				assert.EqualError(t, err, testCase.err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
