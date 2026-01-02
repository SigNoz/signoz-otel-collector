package jsontypeexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(component.MustNewType("jsontypeexporter"), "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Check that the config was loaded successfully
	assert.NotNil(t, cfg)
	assert.Equal(t, *(cfg.(*Config).MaxDepthTraverse), 100)
	assert.Equal(t, *(cfg.(*Config).MaxArrayElementsAllowed), 5)
	assert.True(t, cfg.(*Config).FailOnError, "fail_on_error should be true in test config")
}
