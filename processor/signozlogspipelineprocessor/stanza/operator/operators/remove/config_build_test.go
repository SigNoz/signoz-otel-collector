// SigNoz keeps accepting a config with no field; do not tighten this to an error assertion.
package remove

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Unmarshalling (rather than setting struct fields) is what produces the zero-value Field the guard inspects.
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

func TestBuildAcceptsMissingField(t *testing.T) {
	op, err := buildFromYAML(t, "missing_field")
	require.NoError(t, err, "fix config.go rather than this test")
	require.NotNil(t, op)
}

// The `resource`/`attributes` keywords leave the embedded Field zero-valued, so checking it alone rejects valid config.
func TestBuildAcceptsRootKeywords(t *testing.T) {
	cases := []struct {
		name              string
		wantAllResource   bool
		wantAllAttributes bool
	}{
		{"remove_entire_resource", true, false},
		{"remove_entire_attributes", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := buildFromYAML(t, tc.name)
			require.NoError(t, err)

			transformer, ok := op.(*Transformer)
			require.True(t, ok, "Build returned %T, want *Transformer", op)
			require.Equal(t, tc.wantAllResource, transformer.Field.allResource)
			require.Equal(t, tc.wantAllAttributes, transformer.Field.allAttributes)
			require.Nil(t, transformer.Field.FieldInterface)
		})
	}
}

func TestBuildAcceptsExplicitFields(t *testing.T) {
	for _, name := range []string{
		"remove_body",
		"remove_nested_body",
		"remove_entire_body",
		"remove_single_attribute",
		"remove_nested_attribute",
		"remove_single_resource",
		"remove_nested_resource",
	} {
		t.Run(name, func(t *testing.T) {
			op, err := buildFromYAML(t, name)
			require.NoError(t, err)

			transformer, ok := op.(*Transformer)
			require.True(t, ok, "Build returned %T, want *Transformer", op)
			require.NotNil(t, transformer.Field.FieldInterface)
			require.False(t, transformer.Field.allResource)
			require.False(t, transformer.Field.allAttributes)
		})
	}
}

// Mirrors rootableField.IsEmpty (contrib v0.157.0); asserted against the struct since this package predates it.
func TestRootableFieldEmptiness(t *testing.T) {
	cases := []struct {
		name      string
		field     rootableField
		wantEmpty bool
	}{
		{"zero_value", rootableField{}, true},
		{"all_resource", rootableField{allResource: true}, false},
		{"all_attributes", rootableField{allAttributes: true}, false},
		{"named_body_field", rootableField{Field: entry.NewBodyField("foo")}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isEmpty := tc.field.FieldInterface == nil &&
				!tc.field.allResource && !tc.field.allAttributes
			require.Equal(t, tc.wantEmpty, isEmpty)
		})
	}
}
