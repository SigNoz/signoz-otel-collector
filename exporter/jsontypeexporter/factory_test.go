package jsontypeexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Check that the config is created successfully
	assert.NotNil(t, cfg)
	assert.IsType(t, &Config{}, cfg)
}
