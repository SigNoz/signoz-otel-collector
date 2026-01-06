// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	for _, configType := range []string{
		"cwmetrics", "cwlogs", "otlp_v1", "invalid",
	} {
		t.Run(configType, func(t *testing.T) {
			fileName := configType + "_config.yaml"
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", fileName))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if configType == "invalid" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.Equal(t, &Config{
					RecordType: configType,
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "0.0.0.0:4433",
						TLS: configoptional.Some(configtls.ServerConfig{
							Config: configtls.Config{
								CertFile: "server.crt",
								KeyFile:  "server.key",
							},
						}),
					},
				}, cfg)
			}
		})
	}
}
