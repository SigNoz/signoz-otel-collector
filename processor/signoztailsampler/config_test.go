package signoztailsampler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "tail_sampling_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalProcessorConfig(sub, cfg))

	assert.Equal(t,
		cfg,
		&Config{
			//			ProcessorSettings:       config.NewProcessorSettings(component.NewID(typeStr)),
			DecisionWait:            10 * time.Second,
			NumTraces:               100,
			ExpectedNewTracesPerSec: 10,
			PolicyCfgs: []PolicyCfg{
				{
					Name:     "test-policy-1",
					Type:     PolicyGroup,
					Priority: 1,
					Root:     true,
					ProbabilisticCfg: ProbabilisticCfg{
						SamplingPercentage: 100,
					},
					PolicyFilterCfg: PolicyFilterCfg{
						StringAttributeCfgs: []StringAttributeCfg{
							{Key: "source", Values: []string{"security"}},
						},
					},
					SubPolicies: []PolicyCfg{
						{
							Name:     "condition-1",
							Type:     PolicyGroup,
							Priority: 1,
							PolicyFilterCfg: PolicyFilterCfg{
								StringAttributeCfgs: []StringAttributeCfg{
									{Key: "source", Values: []string{"security"}},
								},
							},
						},
					},
				},
			},
		})
}
