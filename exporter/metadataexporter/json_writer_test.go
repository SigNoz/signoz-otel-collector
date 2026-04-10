package metadataexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func newTestWriter(t *testing.T, cfg JSONConfig) *jsonMetadataWriter {
	t.Helper()
	cache, err := lru.New[string, struct{}](1000)
	require.NoError(t, err)
	return &jsonMetadataWriter{
		cfg:              cfg,
		cardinalKeyCache: cache,
	}
}

func newTestValueAccumulator(t *testing.T) *valueAccumulator {
	t.Helper()
	conn, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	require.NoError(t, err)
	conn.ExpectPrepareBatch(".*")
	batch, err := conn.PrepareBatch(context.Background(), "INSERT INTO noop")
	require.NoError(t, err)
	return &valueAccumulator{
		stmt:             batch,
		shouldSkipFromDB: func(_ string) bool { return false },
		shouldSkipUVT:    func(_ string) bool { return false },
		addToUVT:         func(_ string, _ any) {},
	}
}

func TestWalk_EndToEndTypes(t *testing.T) {
	input := map[string]any{
		"_p": "F",
		"array_objects": []any{
			map[string]any{"a": "Processing event"},
			map[string]any{"x.y": false},
			map[string]any{"p": map[string]any{"q": 65}},
			map[string]any{
				"nested": []any{
					map[string]any{"inside_a": 0.4986468944784865},
					map[string]any{"inside_b": "I am String"},
					map[string]any{"inside_a": false},
				},
				"inbox": []any{"hello", 4.5669},
			},
		},
		"array_objects_and_primitives": []any{
			"Error sending abc webhooks",
			map[string]any{
				"x": "y",
				"nested": []any{
					map[string]any{"message": "hello", "number": 4.5669},
					"hello",
					4.5669,
					false,
				},
			},
		},
		"array_primitives_mixed":     []any{10, "Webhook sent", false, 0.9155561531002926, "hello"},
		"array_primitives_same_type": []any{69, 8, 18, 90, 100},
		"sage":                       map[string]any{"number": "failed450"},
		"created_by":                 "piyushsingariya",
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
		"docker": []any{"container_1", "container_8"},
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

	tests := []struct {
		name     string
		input    map[string]any
		cfg      JSONConfig
		expected map[string][]utils.FieldDataType
	}{
		{
			name: "message_skip_test_simple",
			input: map[string]any{
				"message": map[string]any{"level": "info"},
				"test":    "value",
			},
			cfg: JSONConfig{MaxDepthTraverse: to.Ptr(2), MaxArrayElementsAllowed: to.Ptr(4), MaxKeysAtLevel: to.Ptr(1024)},
			expected: map[string][]utils.FieldDataType{
				"message": {utils.FieldDataTypeString},
				"test":    {utils.FieldDataTypeString},
			},
		},
		{
			name: "message_skip_test_x2",
			input: map[string]any{
				"message.level": "info",
				"test":          "value",
			},
			cfg: JSONConfig{MaxDepthTraverse: to.Ptr(2), MaxArrayElementsAllowed: to.Ptr(4), MaxKeysAtLevel: to.Ptr(1024)},
			expected: map[string][]utils.FieldDataType{
				"test": {utils.FieldDataTypeString},
			},
		},
		{
			name: "simple_datatype_test",
			input: map[string]any{
				"string": []any{"hello"},
				"int":    123,
				"float":  123.456,
				"bool":   []any{true},
			},
			cfg: JSONConfig{MaxDepthTraverse: to.Ptr(2), MaxArrayElementsAllowed: to.Ptr(4), MaxKeysAtLevel: to.Ptr(1024)},
			expected: map[string][]utils.FieldDataType{
				"string": {utils.FieldDataTypeArrayString},
				"int":    {utils.FieldDataTypeInt64},
				"float":  {utils.FieldDataTypeFloat64},
				"bool":   {utils.FieldDataTypeArrayBool},
			},
		},
		{
			name:  "full_test",
			input: input,
			cfg:   JSONConfig{MaxDepthTraverse: to.Ptr(100), MaxArrayElementsAllowed: to.Ptr(5), MaxKeysAtLevel: to.Ptr(1024)},
			expected: map[string][]utils.FieldDataType{
				"_p":                                              {utils.FieldDataTypeString},
				"array_objects":                                   {utils.FieldDataTypeArrayJSON},
				"array_objects[].a":                               {utils.FieldDataTypeString},
				"array_objects[].x.y":                             {utils.FieldDataTypeBool},
				"array_objects[].p.q":                             {utils.FieldDataTypeInt64},
				"array_objects[].nested":                          {utils.FieldDataTypeArrayJSON},
				"array_objects[].nested[].inside_a":               {utils.FieldDataTypeBool, utils.FieldDataTypeFloat64},
				"array_objects[].nested[].inside_b":               {utils.FieldDataTypeString},
				"array_objects[].inbox":                           {utils.FieldDataTypeArrayDynamic},
				"array_objects_and_primitives":                    {utils.FieldDataTypeArrayDynamic},
				"array_objects_and_primitives[].x":                {utils.FieldDataTypeString},
				"array_objects_and_primitives[].nested":           {utils.FieldDataTypeArrayDynamic},
				"array_objects_and_primitives[].nested[].message": {utils.FieldDataTypeString},
				"array_objects_and_primitives[].nested[].number":  {utils.FieldDataTypeFloat64},
				"array_primitives_mixed":                          {utils.FieldDataTypeArrayDynamic},
				"array_primitives_same_type":                      {utils.FieldDataTypeArrayInt64},
				"sage.number":                                     {utils.FieldDataTypeString},
				"created_by":                                      {utils.FieldDataTypeString},
				"details.game.beta-tester":                        {utils.FieldDataTypeBool},
				"details.game.is_game":                            {utils.FieldDataTypeString},
				"details.game.marked_favourite":                   {utils.FieldDataTypeBool},
				"details.game.play_time_hours":                    {utils.FieldDataTypeFloat64},
				"details.game.metadata.installation_path":         {utils.FieldDataTypeString},
				"details.game.metadata.drm.hash_check_status":     {utils.FieldDataTypeString},
				"details.game.metadata.drm.malformed_hardware":    {utils.FieldDataTypeBool},
				"details.game.metadata.drm.running":               {utils.FieldDataTypeBool},
				"details.game.metadata.drm.version":               {utils.FieldDataTypeString},
				"details.game.metadata.version":                   {utils.FieldDataTypeString},
				"details.uninstall":                               {utils.FieldDataTypeBool},
				"docker":                                          {utils.FieldDataTypeArrayString},
				"kubernetes.container_image":                      {utils.FieldDataTypeString},
				"kubernetes.container_name":                       {utils.FieldDataTypeString},
				"kubernetes.docker_id":                            {utils.FieldDataTypeString},
				"kubernetes.host":                                 {utils.FieldDataTypeString},
				"kubernetes.namespace_name":                       {utils.FieldDataTypeString},
				"kubernetes.pod_id":                               {utils.FieldDataTypeString},
				"kubernetes.pod_name":                             {utils.FieldDataTypeString},
				"log":                                             {utils.FieldDataTypeString},
				"log_processed.level":                             {utils.FieldDataTypeString},
				"log_processed.message":                           {utils.FieldDataTypeString},
				"log_processed.target":                            {utils.FieldDataTypeString},
				"log_processed.timestamp":                         {utils.FieldDataTypeString},
				"message":                                         {utils.FieldDataTypeString},
				"stream":                                          {utils.FieldDataTypeString},
				"uninstall":                                       {utils.FieldDataTypeBool},
			},
		},
		{
			name:  "max_depth_traverse_test",
			input: input,
			cfg:   JSONConfig{MaxDepthTraverse: to.Ptr(2), MaxArrayElementsAllowed: to.Ptr(4), MaxKeysAtLevel: to.Ptr(1024)},
			expected: map[string][]utils.FieldDataType{
				"_p":                               {utils.FieldDataTypeString},
				"array_objects":                    {utils.FieldDataTypeArrayJSON},
				"array_objects[].a":                {utils.FieldDataTypeString},
				"array_objects[].x.y":              {utils.FieldDataTypeBool},
				"array_objects_and_primitives":     {utils.FieldDataTypeArrayDynamic},
				"array_objects_and_primitives[].x": {utils.FieldDataTypeString},
				"created_by":                       {utils.FieldDataTypeString},
				"details.uninstall":                {utils.FieldDataTypeBool},
				"docker":                           {utils.FieldDataTypeArrayString},
				"kubernetes.container_image":       {utils.FieldDataTypeString},
				"kubernetes.container_name":        {utils.FieldDataTypeString},
				"kubernetes.docker_id":             {utils.FieldDataTypeString},
				"kubernetes.host":                  {utils.FieldDataTypeString},
				"kubernetes.namespace_name":        {utils.FieldDataTypeString},
				"kubernetes.pod_id":                {utils.FieldDataTypeString},
				"kubernetes.pod_name":              {utils.FieldDataTypeString},
				"log":                              {utils.FieldDataTypeString},
				"log_processed.level":              {utils.FieldDataTypeString},
				"log_processed.message":            {utils.FieldDataTypeString},
				"log_processed.target":             {utils.FieldDataTypeString},
				"log_processed.timestamp":          {utils.FieldDataTypeString},
				"message":                          {utils.FieldDataTypeString},
				"sage.number":                      {utils.FieldDataTypeString},
				"stream":                           {utils.FieldDataTypeString},
				"uninstall":                        {utils.FieldDataTypeBool},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWriter(t, tc.cfg)
			va := newTestValueAccumulator(t)

			body := pcommon.NewValueEmpty()
			require.NoError(t, body.FromRaw(tc.input))

			ts := &typesAccumulator{}
			err := w.walkNode(context.Background(), "", body, 0, utils.TagTypeBodyField, 0, ts, va)
			require.NoError(t, err)

			got := map[string][]utils.FieldDataType{}
			ts.types.Range(func(key, value any) bool {
				path := key.(string)
				cs := value.(*typesConcurrentSet)
				types := cs.Keys()
				sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
				got[path] = types
				return true
			})

			if len(got) != len(tc.expected) {
				buf := bytes.NewBuffer(nil)
				json.NewEncoder(buf).Encode(got)
				t.Log(buf.String())
				t.Fatalf("got %d paths, expected %d", len(got), len(tc.expected))
			}

			for path, want := range tc.expected {
				gotTypes, ok := got[path]
				if !ok {
					t.Fatalf("missing path in got: %s", path)
				}
				sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
				assert.ElementsMatch(t, want, gotTypes, fmt.Sprintf("mismatch at path %s", path))
			}
		})
	}
}

func TestWalk_InferArrayMask(t *testing.T) {
	tests := []struct {
		name  string
		types []pcommon.ValueType
		want  uint16
	}{
		{name: "bool_only", types: []pcommon.ValueType{pcommon.ValueTypeBool, pcommon.ValueTypeBool}, want: maskArrayBool},
		{name: "int_and_float", types: []pcommon.ValueType{pcommon.ValueTypeInt, pcommon.ValueTypeDouble}, want: maskArrayFloat},
		{name: "int_and_bool", types: []pcommon.ValueType{pcommon.ValueTypeInt, pcommon.ValueTypeBool}, want: maskArrayInt},
		{name: "bool_and_float", types: []pcommon.ValueType{pcommon.ValueTypeBool, pcommon.ValueTypeDouble}, want: maskArrayFloat},
		{name: "string_and_int_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeStr, pcommon.ValueTypeInt}, want: maskArrayDynamic},
		{name: "string_and_bool_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeStr, pcommon.ValueTypeBool}, want: maskArrayDynamic},
		{name: "string_and_float_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeBytes, pcommon.ValueTypeDouble}, want: maskArrayDynamic},
		{name: "string_only", types: []pcommon.ValueType{pcommon.ValueTypeStr}, want: maskArrayString},
		{name: "bytes_only", types: []pcommon.ValueType{pcommon.ValueTypeBytes}, want: maskArrayString},
		{name: "json_only", types: []pcommon.ValueType{pcommon.ValueTypeMap}, want: maskArrayJSON},
		{name: "json_and_int_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeMap, pcommon.ValueTypeInt}, want: maskArrayDynamic},
		{name: "json_and_string_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeMap, pcommon.ValueTypeStr}, want: maskArrayDynamic},
		{name: "duplicates_int_only", types: []pcommon.ValueType{pcommon.ValueTypeInt, pcommon.ValueTypeInt}, want: maskArrayInt},
		{name: "duplicates_string_only", types: []pcommon.ValueType{pcommon.ValueTypeStr, pcommon.ValueTypeStr, pcommon.ValueTypeBytes}, want: maskArrayString},
		{name: "duplicates_string_and_int_dynamic", types: []pcommon.ValueType{pcommon.ValueTypeStr, pcommon.ValueTypeStr, pcommon.ValueTypeInt}, want: maskArrayDynamic},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, inferArrayMask(tc.types))
		})
	}
}
