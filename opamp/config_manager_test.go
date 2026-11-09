package opamp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// copyTestdata copies a testdata file into t.TempDir(), since NewDynamicConfig and config reloads rewrite the file in place.
func copyTestdata(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, contents, 0o644))
	return path
}

func newLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return logger
}

func TestNewDynamicConfigInvalidPath(t *testing.T) {
	cnt := 0
	reloadFunc := func(contents []byte) error {
		cnt++
		return nil
	}

	_, err := NewDynamicConfig("./testdata/collector.yaml", reloadFunc, nil)
	assert.ErrorContains(t, err, "no such file or directory")
}

func TestNewDynamicConfig(t *testing.T) {
	cnt := 0
	reloadFunc := func(contents []byte) error {
		cnt++
		return nil
	}

	_, err := NewDynamicConfig(copyTestdata(t, "coll-config-path.yaml"), reloadFunc, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, cnt)
}

func TestNewAgentConfigManager(t *testing.T) {
	logger := newLogger(t)
	mgr := NewAgentConfigManager(logger)
	assert.NotNil(t, mgr)
}

func TestNewAgentConfigManagerEffectiveConfig(t *testing.T) {
	logger := newLogger(t)
	mgr := NewAgentConfigManager(logger)
	assert.NotNil(t, mgr)

	cnt := 0
	reloadFunc := func(contents []byte) error {
		cnt++
		return nil
	}

	path := copyTestdata(t, "coll-config-path.yaml")
	cfg, err := NewDynamicConfig(path, reloadFunc, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, cnt)

	mgr.Set(cfg)
	effCfg, err := mgr.CreateEffectiveConfigMsg(nil)
	assert.NoError(t, err)
	assert.NotNil(t, effCfg)
	bytes, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, effCfg.GetConfigMap().ConfigMap["collector.yaml"].GetContentType(), "text/yaml")
	assert.Equal(t, effCfg.GetConfigMap().ConfigMap["collector.yaml"].Body, bytes)
}

func TestNewDynamicConfigAddsInstanceId(t *testing.T) {
	path := copyTestdata(t, "service-instance-id.yaml")
	_, err := NewDynamicConfig(path, func(contents []byte) error { return nil }, nil)
	assert.NoError(t, err)

	bytes, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(bytes), "service.instance.id: ")
}

func TestNewAgentConfigManagerApply(t *testing.T) {
	logger := newLogger(t)
	mgr := NewAgentConfigManager(logger)
	assert.NotNil(t, mgr)
	mgr.initialConfigReceived = true

	cnt := 0
	reloadFunc := func(contents []byte) error {
		cnt++
		return nil
	}

	cfg, err := NewDynamicConfig(copyTestdata(t, "coll-config-path.yaml"), reloadFunc, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, cnt)

	mgr.Set(cfg)
	effCfg, err := mgr.CreateEffectiveConfigMsg(nil)
	assert.NoError(t, err)
	assert.NotNil(t, effCfg)

	// Apply the same config again
	changed, err := mgr.Apply(&protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: effCfg.GetConfigMap().ConfigMap,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, false, changed)
	assert.Equal(t, 0, cnt)

	newContent, err := os.ReadFile("./testdata/coll-config-path-changed.yaml")
	assert.NoError(t, err)
	newEffCfg := &protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"collector.yaml": {
					ContentType: "text/yaml",
					Body:        newContent,
				},
			},
		},
	}

	// Apply a different config
	changed, err = mgr.Apply(newEffCfg)
	assert.NoError(t, err)
	assert.Equal(t, true, changed)
	assert.Equal(t, 1, cnt)
}
