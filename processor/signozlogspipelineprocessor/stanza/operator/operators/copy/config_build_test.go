// SigNoz keeps accepting configs with a missing from/to; do not tighten these to error assertions.
package copy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

// Unmarshalling (rather than setting struct fields) is what produces the zero-value Field the guards inspect.
func buildFromYAML(t *testing.T, name string) (any, error) {
	t.Helper()

	confMaps, err := confmaptest.LoadConf(filepath.Join(".", "testdata", "config.yaml"))
	require.NoError(t, err)

	sub, err := confMaps.Sub(name)
	require.NoError(t, err)
	require.NotZero(t, len(sub.AllKeys()), "config not found: %q", name)

	cfg := NewConfig()
	require.NoError(t, sub.Unmarshal(cfg))

	cfg.OutputIDs = []string{"fake"}
	return cfg.Build(componenttest.NewNopTelemetrySettings())
}

func TestBuildAcceptsMissingFields(t *testing.T) {
	for _, name := range []string{"missing_from", "missing_to", "missing_both"} {
		t.Run(name, func(t *testing.T) {
			op, err := buildFromYAML(t, name)
			require.NoError(t, err, "fix config.go rather than this test")
			require.NotNil(t, op)
		})
	}
}

func TestBuildAcceptsValidFields(t *testing.T) {
	for _, name := range []string{
		"body_to_body",
		"body_to_attribute",
		"attribute_to_body",
		"attribute_to_resource",
		"attribute_to_nested_attribute",
		"resource_to_nested_resource",
	} {
		t.Run(name, func(t *testing.T) {
			op, err := buildFromYAML(t, name)
			require.NoError(t, err)

			transformer, ok := op.(*Transformer)
			require.True(t, ok, "Build returned %T, want *Transformer", op)
			require.NotNil(t, transformer.From.FieldInterface)
			require.NotNil(t, transformer.To.FieldInterface)
		})
	}
}
