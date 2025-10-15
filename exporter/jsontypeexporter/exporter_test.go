package jsontypeexporter

import (
	"context"
	"sort"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Single end-to-end test: build a representative log body map and compare final path->types.
func TestAnalyzePValue_EndToEndTypes(t *testing.T) {
	exp := &jsonTypeExporter{}
	ctx := context.Background()

	// Construct log body as an actual map[string]any
	input := map[string]any{
		"_p": "F",
		"array_objects": []any{
			map[string]any{
				"a": "Processing event",
			},
			map[string]any{
				"x.y": false,
			},
			map[string]any{
				"p": map[string]any{
					"q": 65,
				},
			},
			map[string]any{
				"nested": []any{
					map[string]any{
						"inside_a": 0.4986468944784865,
					},
					map[string]any{
						"inside_b": "I am String",
					},
					map[string]any{
						"inside_a": false,
					},
				},
				"inbox": []any{
					"hello",
					4.5669,
				},
			},
		},
		"array_objects_and_primitives": []any{
			"Error sending abc webhooks",
			map[string]any{
				"x": "y",
				"nested": []any{
					map[string]any{
						"message": "hello",
						"number":  4.5669,
					},
					"hello",
					4.5669,
					false,
				},
			},
		},
		"array_primitives_mixed": []any{
			10,
			"Webhook sent",
			false,
			0.9155561531002926,
		},
		"array_primitives_same_type": []any{
			69,
			8,
			18,
			90,
		},
		"sage": map[string]any{
			"number": "failed450",
		},
		"created_by": "piyushsingariya",
		"details": map[string]any{
			"game": map[string]any{
				"is_game": "false",
				"metadata": map[string]any{
					"installation_path": "/opt/games/witcher3",
					"drm": map[string]any{
						"hash_check_status":  "success",
						"malformed_hardware": false,
						"running":            false,
						"version":            "patch_v1.101.0",
					},
					"version": "v0.0.3",
				},
			},
			"uninstall": true,
		},
		"docker": []any{
			"container_1",
			"container_8",
		},
		"kubernetes": map[string]any{
			"container_image": "some-image",
			"container_name":  "witcher2-0000-01",
			"docker_id":       "10fe04f01bb9d2ba",
			"host":            "ip-42-96-24-40.ap-south-1.compute.internal",
			"namespace_name":  "prod",
			"pod_id":          "1feea36b1ff05767",
			"pod_name":        "aws-integration-agent-00-1",
		},
		"log": "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::3rdevent_provider_ebpf\"}",
		"log_processed": map[string]any{
			"level":     "DEBUG",
			"message":   "Processing event",
			"target":    "amzn_nfm::events::event_provider_ebpf",
			"timestamp": "1753769510807",
		},
		"message":   "under valorant 3",
		"stream":    "stdout",
		"uninstall": false,
	}

	body := pcommon.NewValueEmpty()
	require.NoError(t, body.FromRaw(input))

	// Collect bitmasks via analyzePValue
	typeSet := TypeSet{}
	err := exp.analyzePValue(ctx, "", false, body, &typeSet)
	require.NoError(t, err)

	// Expand masks to final type strings (same mapping as in pushLogs)
	got := map[string][]string{}
	typeSet.types.Range(func(key, value interface{}) bool {
		path := key.(string)
		cs := value.(*utils.ConcurrentSet[string])
		types := cs.Keys()
		sort.Strings(types)
		got[path] = types
		return true
	})

	// Expected final types per path (subset assertion)
	expected := map[string][]string{
		"_p":                                           {StringType},
		"array_objects":                                {ArrayJSON},
		"array_objects:a":                              {StringType},
		"array_objects:x.y":                            {BooleanType},
		"array_objects:p.q":                            {IntType},
		"array_objects:nested":                         {ArrayJSON},
		"array_objects:nested:inside_a":                {BooleanType, Float64Type},
		"array_objects:nested:inside_b":                {StringType},
		"array_objects:inbox":                          {ArrayDynamic},
		"array_objects_and_primitives":                 {ArrayDynamic},
		"array_objects_and_primitives:x":               {StringType},
		"array_objects_and_primitives:nested":          {ArrayDynamic},
		"array_objects_and_primitives:nested:message":  {StringType},
		"array_objects_and_primitives:nested:number":   {Float64Type},
		"array_primitives_mixed":                       {ArrayDynamic},
		"array_primitives_same_type":                   {ArrayInt},
		"sage.number":                                  {StringType},
		"created_by":                                   {StringType},
		"details.game.is_game":                         {StringType},
		"details.game.metadata.installation_path":      {StringType},
		"details.game.metadata.drm.hash_check_status":  {StringType},
		"details.game.metadata.drm.malformed_hardware": {BooleanType},
		"details.game.metadata.drm.running":            {BooleanType},
		"details.game.metadata.drm.version":            {StringType},
		"details.game.metadata.version":                {StringType},
		"details.uninstall":                            {BooleanType},
		"docker":                                       {ArrayString},
		"kubernetes.container_image":                   {StringType},
		"kubernetes.container_name":                    {StringType},
		"kubernetes.docker_id":                         {StringType},
		"kubernetes.host":                              {StringType},
		"kubernetes.namespace_name":                    {StringType},
		"kubernetes.pod_id":                            {StringType},
		"kubernetes.pod_name":                          {StringType},
		"log":                                          {StringType},
		"log_processed.level":                          {StringType},
		"log_processed.message":                        {StringType},
		"log_processed.target":                         {StringType},
		"log_processed.timestamp":                      {StringType},
		"message":                                      {StringType},
		"stream":                                       {StringType},
		"uninstall":                                    {BooleanType},
	}

	if len(got) != len(expected) {
		t.Fatalf("got %d paths, expected %d", len(got), len(expected))
	}

	// Assert that expected is a subset of got (types order-insensitive)
	for path, want := range expected {
		gotTypes, ok := got[path]
		if !ok {
			t.Fatalf("missing path in got: %s", path)
		}
		sort.Strings(want)
		assert.ElementsMatch(t, want, gotTypes, "mismatch at path %s", path)
	}
}
