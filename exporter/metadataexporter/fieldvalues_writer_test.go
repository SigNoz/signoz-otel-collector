package metadataexporter

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvalues"
	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

// captureBatch records the values that the JSON writer appends.
type captureBatch struct {
	driver.Batch
	values []string
}

func (c *captureBatch) Append(v ...any) error {
	path := v[1].(string)
	switch {
	case v[4] != nil:
		c.values = append(c.values, path+"="+v[4].(string))
	case v[5] != nil:
		c.values = append(c.values, path+"="+strconv.FormatFloat(v[5].(float64), 'g', -1, 64))
	default:
		c.values = append(c.values, path+"=bool")
	}
	return nil
}

// The field values writer walks JSON bodies itself. It must find the same
// paths and values as the JSON writer, and the same keys with the same data
// types, so that body suggestions and body keys do not change when the store
// replaces the tag table and field_keys.
func TestFieldValuesBodyWalkMatchesTheJSONWriter(t *testing.T) {
	deep := map[string]any{"leaf": "too deep"}
	for i := 0; i < 12; i++ {
		deep = map[string]any{"d": deep}
	}
	bodies := []map[string]any{
		{
			"_p": "F",
			"array_objects": []any{
				map[string]any{"a": "Processing event"},
				map[string]any{"x.y": false},
				map[string]any{"p": map[string]any{"q": 65}},
				map[string]any{
					"nested": []any{map[string]any{"inside_a": 0.4986468944784865}, map[string]any{"inside_b": "I am String"}},
					"inbox":  []any{"hello", 4.5669},
				},
			},
			"array_primitives_mixed":               []any{10, "Webhook sent", false, 0.9155561531002926},
			"nested_arrays":                        []any{"a", []any{"b"}},
			"details":                              map[string]any{"game": map[string]any{"is_game": "false", "play_time_hours": 5.5, "beta-tester": true}},
			"log_processed":                        map[string]any{"level": "DEBUG", "message": "Processing event"},
			"message":                              "under valorant 3",
			"empty":                                "",
			"long":                                 strings.Repeat("x", 300),
			"0f8fad5b-d9cb-469f-a165-70867728950e": "an id as a key",
		},
		{"deep": deep, "ok": "yes"},
		{
			"count":         int64(3),
			"ratio":         0.5,
			"nan":           math.NaN(),
			"flag":          true,
			"empty":         "",
			"message":       map[string]any{"text": "a message that is a map"},
			"objects":       []any{map[string]any{"sku": "a1"}, map[string]any{"qty": int64(2)}},
			"mixed":         []any{map[string]any{"sku": "a1"}, "x"},
			"ints":          []any{int64(1), int64(2)},
			"floats":        []any{int64(1), 2.5},
			"bools":         []any{true, false},
			"nan_array":     []any{math.NaN()},
			"empty_strings": []any{"", ""},
			"nulls":         []any{nil, nil},
			"none":          []any{},
			"then_nested":   []any{map[string]any{"before": "walked"}, []any{"b"}, map[string]any{"after": "not walked"}},
			"empty_objects": []any{map[string]any{}},
		},
	}
	cfg := JSONConfig{MaxDepthTraverse: to.Ptr(defaultJSONMaxDepthTraverse), MaxArrayElementsAllowed: to.Ptr(defaultJSONMaxArrayElementsAllowed), MaxKeysAtLevel: to.Ptr(defaultJSONMaxKeysAtLevel)}
	limits := fieldvalues.BodyJSONLimits{MaxDepthTraverse: *cfg.MaxDepthTraverse, MaxArrayElementsAllowed: *cfg.MaxArrayElementsAllowed, MaxKeysAtLevel: *cfg.MaxKeysAtLevel}

	for i, raw := range bodies {
		body := pcommon.NewValueMap()
		require.NoError(t, body.Map().FromRaw(raw))

		capture := &captureBatch{}
		va := &valueAccumulator{
			stmt:             capture,
			shouldSkipFromDB: func(string) bool { return false },
			shouldSkipUVT:    func(string) bool { return false },
			addToUVT:         func(string, any) {},
		}
		jsonWriter := newTestWriter(t, cfg)
		jsonWriter.logger = zap.NewNop()
		types := &typesAccumulator{}
		_, err := jsonWriter.walkNode(context.Background(), "", body, 0, utils.TagTypeBodyField, 0, types, va)
		require.NoError(t, err)

		wantKeys := map[string]struct{}{}
		types.types.Range(func(path, set any) bool {
			for _, dataType := range set.(*typesConcurrentSet).Keys() {
				wantKeys[path.(string)+" "+string(dataType)] = struct{}{}
			}
			return true
		})
		keys := map[string]struct{}{}
		for _, k := range fieldvalues.BodyKeys(body, limits) {
			keys[k] = struct{}{}
		}
		assert.Equal(t, wantKeys, keys, "keys of body %d", i)

		var store []string
		for _, v := range fieldvalues.BodyValues(body, limits) {
			value := v[strings.Index(v, "=")+1:]
			if value == "" || len(value) > common.MaxAttributeValueLength {
				continue
			}
			store = append(store, v)
		}
		// The JSON writer stores NaN as a value. The store keeps the key and
		// no value.
		var want []string
		for _, v := range capture.values {
			if !strings.HasSuffix(v, "=NaN") {
				want = append(want, v)
			}
		}
		assert.ElementsMatch(t, want, store, "body %d", i)
	}
}

func TestFieldValuesWriterIsOffByDefault(t *testing.T) {
	set := exportertest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig().(*Config)
	cfg.Enabled = true
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalTraces, pipeline.SignalMetrics} {
		e, err := newMetadataExporter(context.Background(), *cfg, set, signal)
		require.NoError(t, err)
		assert.Nil(t, e.fieldValues)
		assert.Len(t, e.logsMetadataWriters, 1, "only the attribute writer, as today")
		require.NoError(t, e.Shutdown(context.Background()))
	}
}

func TestFieldValuesWriterIsAddedWhenEnabled(t *testing.T) {
	set := exportertest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig().(*Config)
	cfg.Enabled = true
	cfg.FieldValues.Enabled = true
	cfg.FieldValues.Cache.MaxBytes = 1 << 20

	logs, err := newMetadataExporter(context.Background(), *cfg, set, pipeline.SignalLogs)
	require.NoError(t, err)
	require.NotNil(t, logs.fieldValues)
	require.Len(t, logs.logsMetadataWriters, 2)
	assert.IsType(t, &attributeMetadataWriter{}, logs.logsMetadataWriters[0], "the existing writer stays first and unchanged")
	assert.IsType(t, fieldValuesLogsWriter{}, logs.logsMetadataWriters[1])
	require.NoError(t, logs.Shutdown(context.Background()))

	traces, err := newMetadataExporter(context.Background(), *cfg, set, pipeline.SignalTraces)
	require.NoError(t, err)
	require.NotNil(t, traces.fieldValues)
	assert.Len(t, traces.logsMetadataWriters, 1, "only the logs exporter runs the writer as a logs writer")
	require.NoError(t, traces.Shutdown(context.Background()))

	cfg.Enabled = false
	off, err := newMetadataExporter(context.Background(), *cfg, set, pipeline.SignalLogs)
	require.NoError(t, err)
	assert.Nil(t, off.fieldValues, "the exporter switch still turns everything off")
	require.NoError(t, off.Shutdown(context.Background()))
}
