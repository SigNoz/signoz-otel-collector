package keyauth

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// NewFactory creates a factory for the static bearer token Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.Type("keyauth"),
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Headers: []string{"signoz-access-token"},
	}
}

func createExtension(_ context.Context, settings extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return newKeyAuth(cfg.(*Config), settings.Logger), nil
}
