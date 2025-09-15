package jsontypeexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	expectedCfg := &Config{
		OutputPath: "./output.json",
	}

	// Only check the OutputPath since other fields have default values
	assert.Equal(t, expectedCfg.OutputPath, cfg.(*Config).OutputPath)
}
