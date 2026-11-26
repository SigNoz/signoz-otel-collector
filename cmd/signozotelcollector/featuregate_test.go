package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	signozcolFeatureGate "github.com/SigNoz/signoz-otel-collector/featuregate"
)

// The components are linked into this binary, so every gate in the list must be
// registered and must be off after ApplyDefaults. A gate that upstream removes or
// marks stable makes this test fail.
func TestApplyDefaults(t *testing.T) {
	reg := featuregate.GlobalRegistry()
	require.NoError(t, signozcolFeatureGate.ApplyDefaults(reg))

	enabled := map[string]bool{}
	reg.VisitAll(func(g *featuregate.Gate) {
		enabled[g.ID()] = g.IsEnabled()
	})
	for _, id := range signozcolFeatureGate.DisabledByDefault {
		on, ok := enabled[id]
		assert.True(t, ok, "gate %s is not registered", id)
		assert.False(t, on, "gate %s is enabled", id)
	}

	// The flag still overrides the defaults.
	flag := signozcolFeatureGate.NewFlag(reg)
	require.NoError(t, flag.Set("receiver.kubeletstats.cpuUsageScrapeBased"))
	t.Cleanup(func() { _ = reg.Set("receiver.kubeletstats.cpuUsageScrapeBased", false) })
	reg.VisitAll(func(g *featuregate.Gate) {
		if g.ID() == "receiver.kubeletstats.cpuUsageScrapeBased" {
			assert.True(t, g.IsEnabled())
		}
	})
}
