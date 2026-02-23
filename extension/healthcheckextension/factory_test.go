// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/SigNoz/signoz-otel-collector/extension/healthcheckextension/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/internal/common/testutil"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  "0.0.0.0:13133",
				Transport: "tcp",
			},
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestFactory_CreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr = confignet.AddrConfig{
		Endpoint:  testutil.GetAvailableLocalAddress(t),
		Transport: "tcp",
	}

	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
