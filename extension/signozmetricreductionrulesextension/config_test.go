package signozmetricreductionrulesextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	testCases := []struct {
		name      string
		id        component.ID
		expected  Config
		expectErr string
	}{
		{
			name:     "NoFieldsSet_DefaultsUsed",
			id:       component.NewID(factory.Type()),
			expected: Config{DSN: "tcp://localhost:9000", Database: "signoz_metrics", RulesTable: "distributed_metric_reduction_rules", PollInterval: 45 * time.Second},
		},
		{
			name:     "AllFieldsSet_PollIntervalAtMinimum_Loaded",
			id:       component.NewIDWithName(factory.Type(), "all_fields"),
			expected: Config{DSN: "tcp://clickhouse:9000", Database: "metrics", RulesTable: "metric_reduction_rules", PollInterval: 5 * time.Second},
		},
		{name: "DSNEmpty_Rejected", id: component.NewIDWithName(factory.Type(), "dsn_empty"), expectErr: "dsn must be specified"},
		{name: "RulesTableEmpty_Rejected", id: component.NewIDWithName(factory.Type(), "rules_table_empty"), expectErr: "rules_table must be specified"},
		{name: "PollIntervalBelowMinimum_Rejected", id: component.NewIDWithName(factory.Type(), "poll_interval_below_minimum"), expectErr: "poll_interval must be at least 5s"},
		{name: "UnknownField_Rejected", id: component.NewIDWithName(factory.Type(), "unknown_field"), expectErr: "has invalid keys: table"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(testCase.id.String())
			require.NoError(t, err)
			if testCase.expectErr != "" {
				err := sub.Unmarshal(cfg)
				if err == nil {
					err = xconfmap.Validate(cfg)
				}
				assert.ErrorContains(t, err, testCase.expectErr)
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, testCase.expected, *cfg.(*Config))
		})
	}
}
