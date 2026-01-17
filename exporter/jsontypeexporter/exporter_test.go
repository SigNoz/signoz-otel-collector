package jsontypeexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Single end-to-end test: build a representative log body map and compare final path->types.
func TestAnalyzePValue_EndToEndTypes(t *testing.T) {
	keyCache, err := lru.New[string, struct{}](1000)
	if err != nil {
		t.Fatalf("failed to create key cache: %v", err)
	}
	exp := &jsonTypeExporter{
		keyCache: keyCache,
	}
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
			"hello",
		},
		"array_primitives_same_type": []any{
			69,
			8,
			18,
			90,
			100,
		},
		"sage": map[string]any{
			"number": "failed450",
		},
		"created_by": "piyushsingariya",
		"details": map[string]any{
			"game": map[string]any{
				"is_game":          "false",
				"marked_favourite": true,
				"play_time_hours":  5.5,
				"beta-tester":      true,
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

	testCases := []struct {
		name     string
		config   *Config
		expected map[string][]string
	}{
		{
			name: "full_test",
			config: &Config{
				MaxDepthTraverse:        utils.ToPointer(100),
				MaxArrayElementsAllowed: utils.ToPointer(5),
			},
			expected: map[string][]string{
				"_p":                                              {String},
				"array_objects":                                   {ArrayJSON},
				"array_objects[].a":                               {String},
				"array_objects[].x.y":                             {Bool},
				"array_objects[].p.q":                             {Int64},
				"array_objects[].nested":                          {ArrayJSON},
				"array_objects[].nested[].inside_a":               {Bool, Float64},
				"array_objects[].nested[].inside_b":               {String},
				"array_objects[].inbox":                           {ArrayDynamic},
				"array_objects_and_primitives":                    {ArrayDynamic},
				"array_objects_and_primitives[].x":                {String},
				"array_objects_and_primitives[].nested":           {ArrayDynamic},
				"array_objects_and_primitives[].nested[].message": {String},
				"array_objects_and_primitives[].nested[].number":  {Float64},
				"array_primitives_mixed":                          {ArrayDynamic},
				"array_primitives_same_type":                      {ArrayInt64},
				"sage.number":                                     {String},
				"created_by":                                      {String},
				"details.game.`beta-tester`":                      {Bool},
				"details.game.is_game":                            {String},
				"details.game.marked_favourite":                   {Bool},
				"details.game.play_time_hours":                    {Float64},
				"details.game.metadata.installation_path":         {String},
				"details.game.metadata.drm.hash_check_status":     {String},
				"details.game.metadata.drm.malformed_hardware":    {Bool},
				"details.game.metadata.drm.running":               {Bool},
				"details.game.metadata.drm.version":               {String},
				"details.game.metadata.version":                   {String},
				"details.uninstall":                               {Bool},
				"docker":                                          {ArrayString},
				"kubernetes.container_image":                      {String},
				"kubernetes.container_name":                       {String},
				"kubernetes.docker_id":                            {String},
				"kubernetes.host":                                 {String},
				"kubernetes.namespace_name":                       {String},
				"kubernetes.pod_id":                               {String},
				"kubernetes.pod_name":                             {String},
				"log":                                             {String},
				"log_processed.level":                             {String},
				"log_processed.message":                           {String},
				"log_processed.target":                            {String},
				"log_processed.timestamp":                         {String},
				"message":                                         {String},
				"stream":                                          {String},
				"uninstall":                                       {Bool},
			},
		},
		{
			name: "max_depth_traverse_test",
			config: &Config{
				MaxDepthTraverse:        utils.ToPointer(2),
				MaxArrayElementsAllowed: utils.ToPointer(4),
			},
			expected: map[string][]string{
				"_p":                               {String},
				"array_objects":                    {ArrayJSON},
				"array_objects[].a":                {String},
				"array_objects[].x.y":              {Bool},
				"array_objects_and_primitives":     {ArrayDynamic},
				"array_objects_and_primitives[].x": {String},
				"created_by":                       {String},
				"details.uninstall":                {Bool},
				"docker":                           {ArrayString},
				"kubernetes.container_image":       {String},
				"kubernetes.container_name":        {String},
				"kubernetes.docker_id":             {String},
				"kubernetes.host":                  {String},
				"kubernetes.namespace_name":        {String},
				"kubernetes.pod_id":                {String},
				"kubernetes.pod_name":              {String},
				"log":                              {String},
				"log_processed.level":              {String},
				"log_processed.message":            {String},
				"log_processed.target":             {String},
				"log_processed.timestamp":          {String},
				"message":                          {String},
				"sage.number":                      {String},
				"stream":                           {String},
				"uninstall":                        {Bool},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exp.config = testCase.config
			// Collect bitmasks via analyzePValue
			typeSet := TypeSet{}
			err := exp.analyzePValue(ctx, body, &typeSet)
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
			if len(got) != len(testCase.expected) {
				buf := bytes.NewBuffer(nil)
				json.NewEncoder(buf).Encode(got)
				t.Log(buf.String())
				t.Fatalf("got %d paths, expected %d", len(got), len(testCase.expected))
			}

			// Assert that expected is a subset of got (types order-insensitive)
			for path, want := range testCase.expected {
				gotTypes, ok := got[path]
				if !ok {
					t.Fatalf("missing path in got: %s", path)
				}
				sort.Strings(want)
				assert.ElementsMatch(t, want, gotTypes, fmt.Sprintf("mismatch at path %s", path))
			}
		})
	}
}
