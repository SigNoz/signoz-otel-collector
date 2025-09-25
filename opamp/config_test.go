package opamp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfigInvalidPath(t *testing.T) {
	_, err := ParseAgentManagerConfig("./testdata/collector.yaml")
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestParseConfigInvalidYaml(t *testing.T) {
	cfg, err := ParseAgentManagerConfig("./testdata/invalid.yaml")
	if err == nil {
		t.Errorf("expected error")
	}
	if cfg != nil {
		t.Errorf("expected nil config but got %v", cfg)
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := ParseAgentManagerConfig("./testdata/manager-config.yaml")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Errorf("expected config")
	}
}

func TestParseConfigAddsID(t *testing.T) {
	// make a copy of the file
	func() {
		err := copy("./testdata/agent-id.yaml", "./testdata/agent-id-copy.yaml")
		assert.NoError(t, err, "failed to copy file")
	}()

	// restore the original file
	defer func() {
		err := copy("./testdata/agent-id-copy.yaml", "./testdata/agent-id.yaml")
		assert.NoError(t, err, "failed to restore original file")
		os.Remove("./testdata/agent-id-copy.yaml")
	}()

	cfg, err := ParseAgentManagerConfig("./testdata/agent-id.yaml")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.NotNil(t, cfg)
	if cfg.ID == "" {
		t.Errorf("expected agent ID to be set")
	}
}
