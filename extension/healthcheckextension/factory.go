// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/SigNoz/signoz-otel-collector/extension/healthcheckextension/internal/metadata"
)

const defaultPort = 13133

// NewFactory creates a factory for HealthCheck extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: fmt.Sprintf("0.0.0.0:%d", defaultPort),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)

	return newServer(*config, set.TelemetrySettings), nil
}

// defaultCheckCollectorPipelineSettings returns the default settings for CheckCollectorPipeline.
func defaultCheckCollectorPipelineSettings() checkCollectorPipelineSettings {
	return checkCollectorPipelineSettings{
		Enabled:                  false,
		Interval:                 "5m",
		ExporterFailureThreshold: 5,
	}
}
