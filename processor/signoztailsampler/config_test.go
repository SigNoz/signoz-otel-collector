// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tailsamplingprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
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
			ProcessorSettings:       config.NewProcessorSettings(component.NewID(typeStr)),
			DecisionWait:            10 * time.Second,
			NumTraces:               100,
			ExpectedNewTracesPerSec: 10,
			PolicyCfgs: []PolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-policy-1",
						Type: AlwaysSample,
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-policy-2",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 5000},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:                "test-policy-3",
						Type:                NumericAttribute,
						NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:             "test-policy-4",
						Type:             Probabilistic,
						ProbabilisticCfg: ProbabilisticCfg{HashSalt: "custom-salt", SamplingPercentage: 0.1},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:          "test-policy-5",
						Type:          StatusCode,
						StatusCodeCfg: StatusCodeCfg{StatusCodes: []string{"ERROR", "UNSET"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:               "test-policy-6",
						Type:               StringAttribute,
						StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:            "test-policy-7",
						Type:            RateLimiting,
						RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 35},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:         "test-policy-8",
						Type:         SpanCount,
						SpanCountCfg: SpanCountCfg{MinSpans: 2},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:          "test-policy-9",
						Type:          TraceState,
						TraceStateCfg: TraceStateCfg{Key: "key3", Values: []string{"value1", "value2"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "and-policy-1",
						Type: And,
					},
					AndCfg: AndCfg{
						SubPolicyCfg: []AndSubPolicyCfg{
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:                "test-and-policy-1",
									Type:                NumericAttribute,
									NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:               "test-and-policy-2",
									Type:               StringAttribute,
									StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
								},
							},
						},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "composite-policy-1",
						Type: Composite,
					},
					CompositeCfg: CompositeCfg{
						MaxTotalSpansPerSecond: 1000,
						PolicyOrder:            []string{"test-composite-policy-1", "test-composite-policy-2", "test-composite-policy-3"},
						SubPolicyCfg: []CompositeSubPolicyCfg{
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:                "test-composite-policy-1",
									Type:                NumericAttribute,
									NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:               "test-composite-policy-2",
									Type:               StringAttribute,
									StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name: "test-composite-policy-3",
									Type: AlwaysSample,
								},
							},
						},
						RateAllocation: []RateAllocationCfg{
							{
								Policy:  "test-composite-policy-1",
								Percent: 50,
							},
							{
								Policy:  "test-composite-policy-2",
								Percent: 25,
							},
						},
					},
				},
			},
		})
}
