package opamp

import (
	"testing"
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
