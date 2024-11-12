package clickhousetracesexporter

import (
	"reflect"
	"sort"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_attributesData_add(t *testing.T) {

	type args struct {
		key   string
		value pcommon.Value
	}
	tests := []struct {
		name   string
		args   []args
		result attributesData
	}{
		{
			name: "test_string",
			args: []args{
				{
					key:   "test_key",
					value: pcommon.NewValueStr("test_string"),
				},
			},
			result: attributesData{
				StringMap: map[string]string{
					"test_key": "test_string",
				},
				NumberMap: map[string]float64{},
				BoolMap:   map[string]bool{},
				SpanAttributes: []SpanAttribute{
					{
						Key:         "test_key",
						TagType:     "tag",
						IsColumn:    false,
						StringValue: "test_string",
						DataType:    "string",
					},
				},
			},
		},
		{
			name: "test_all_type",
			args: []args{
				{
					key:   "double",
					value: pcommon.NewValueDouble(10.0),
				},
				{
					key:   "integer",
					value: pcommon.NewValueInt(10),
				},
				{
					key:   "bool",
					value: pcommon.NewValueBool(true),
				},
				{
					key: "map",
					value: func() pcommon.Value {
						v := pcommon.NewValueMap()
						m := v.Map()
						m.PutStr("nested_key", "nested_value")
						m.PutDouble("nested_double", 20.5)
						m.PutBool("nested_bool", false)
						return v
					}(),
				},
			},
			result: attributesData{
				StringMap: map[string]string{
					"nested_key": "nested_value",
				},
				NumberMap: map[string]float64{
					"double":        10.0,
					"integer":       10.0,
					"nested_double": 20.5,
				},
				BoolMap: map[string]bool{
					"bool":        true,
					"nested_bool": false,
				},
				SpanAttributes: []SpanAttribute{
					{
						Key:         "nested_key",
						TagType:     "tag",
						IsColumn:    false,
						StringValue: "nested_value",
						DataType:    "string",
					},
					{
						Key:         "double",
						TagType:     "tag",
						IsColumn:    false,
						NumberValue: 10.0,
						DataType:    "float64",
					},
					{
						Key:         "integer",
						TagType:     "tag",
						IsColumn:    false,
						NumberValue: 10.0,
						DataType:    "float64",
					},
					{
						Key:         "nested_double",
						TagType:     "tag",
						IsColumn:    false,
						NumberValue: 20.5,
						DataType:    "float64",
					},
					{
						Key:      "bool",
						TagType:  "tag",
						IsColumn: false,
						DataType: "bool",
					},
					{
						Key:      "nested_bool",
						TagType:  "tag",
						IsColumn: false,
						DataType: "bool",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrMap := attributesData{
				StringMap:      map[string]string{},
				NumberMap:      map[string]float64{},
				BoolMap:        map[string]bool{},
				SpanAttributes: []SpanAttribute{},
			}
			for _, arg := range tt.args {
				attrMap.add(arg.key, arg.value)
			}
			if !reflect.DeepEqual(tt.result.StringMap, attrMap.StringMap) {
				t.Errorf("StringMap mismatch: expected %v, got %v", tt.result.StringMap, attrMap.StringMap)
			}
			if !reflect.DeepEqual(tt.result.NumberMap, attrMap.NumberMap) {
				t.Errorf("NumberMap mismatch: expected %v, got %v", tt.result.NumberMap, attrMap.NumberMap)
			}
			if !reflect.DeepEqual(tt.result.BoolMap, attrMap.BoolMap) {
				t.Errorf("BoolMap mismatch: expected %v, got %v", tt.result.BoolMap, attrMap.BoolMap)
			}

			// For SpanAttributes, need to sort both slices first since order doesn't matter
			expectedAttrs := make([]SpanAttribute, len(tt.result.SpanAttributes))
			actualAttrs := make([]SpanAttribute, len(attrMap.SpanAttributes))
			copy(expectedAttrs, tt.result.SpanAttributes)
			copy(actualAttrs, attrMap.SpanAttributes)

			sort.Slice(expectedAttrs, func(i, j int) bool {
				return expectedAttrs[i].Key < expectedAttrs[j].Key
			})
			sort.Slice(actualAttrs, func(i, j int) bool {
				return actualAttrs[i].Key < actualAttrs[j].Key
			})

			if !reflect.DeepEqual(expectedAttrs, actualAttrs) {
				t.Errorf("SpanAttributes mismatch: expected %v, got %v", expectedAttrs, actualAttrs)
			}
		})
	}
}
