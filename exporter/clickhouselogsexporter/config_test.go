// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouselogsexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 3)

	defaultCfg := factory.CreateDefaultConfig()
	defaultCfg.(*Config).DSN = "tcp://127.0.0.1:9000/?dial_timeout=5s"
	r0 := cfg.Exporters[component.NewID(metadata.Type)]
	assert.Equal(t, r0, defaultCfg)

	r1 := cfg.Exporters[component.NewIDWithName(metadata.Type, "full")].(*Config)
	expectedConfig := &Config{
		DSN: "tcp://127.0.0.1:9000/?dial_timeout=5s",
		TimeoutConfig: exporterhelper.TimeoutConfig{
			Timeout: 5 * time.Second,
		},
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      300 * time.Second,
			RandomizationFactor: 0.7,
			Multiplier:          1.3,
		},
		QueueBatchConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
			Sizer:        exporterhelper.RequestSizerTypeRequests,
			NumConsumers: 10,
			QueueSize:    100,
			Batch: configoptional.Default(exporterhelper.BatchConfig{
				FlushTimeout: 200 * time.Millisecond,
				Sizer:        exporterhelper.RequestSizerTypeItems,
				MinSize:      8192,
			}),
		}),
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: 10 * time.Minute,
			MaxDistinctValues: 25000,
		},
		LogLevelConcurrency:       utils.ToPointer(7),
		BodyJSONEnabled:           true,
		PromotedPathsSyncInterval: utils.ToPointer(10 * time.Second),
		BodyJSONOldBodyEnabled:    true,
		MaxAllowedDataAgeDays:     utils.ToPointer(15),
	}
	assert.Equal(t, expectedConfig, r1)

	defaultCfg.(*Config).UseNewSchema = true
	r2 := cfg.Exporters[component.NewIDWithName(metadata.Type, "new_schema")]
	assert.Equal(t, r2, defaultCfg)
}
