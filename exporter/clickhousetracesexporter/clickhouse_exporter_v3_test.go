package clickhousetracesexporter

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
					"map.nested_key": "nested_value",
				},
				NumberMap: map[string]float64{
					"double":            10.0,
					"integer":           10.0,
					"map.nested_double": 20.5,
				},
				BoolMap: map[string]bool{
					"bool":            true,
					"map.nested_bool": false,
				},
				SpanAttributes: []SpanAttribute{
					{
						Key:         "map.nested_key",
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
						Key:         "map.nested_double",
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
						Key:      "map.nested_bool",
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

func Test_populateEventsV3(t *testing.T) {
	type args struct {
		events                       ptrace.SpanEventSlice
		span                         *SpanV3
		lowCardinalExceptionGrouping bool
	}
	tests := []struct {
		name   string
		args   args
		result SpanV3
	}{
		{
			name: "test_exception",
			args: args{
				events: func() ptrace.SpanEventSlice {
					events := ptrace.NewSpanEventSlice()
					event := events.AppendEmpty()
					event.SetName("exception")
					event.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
					attrs := event.Attributes()
					attrs.PutStr("exception.type", "RuntimeError")
					attrs.PutStr("exception.message", "Something went wrong")
					attrs.PutStr("exception.stacktrace", "at line 42\nat line 43")
					return events
				}(),
				span:                         &SpanV3{},
				lowCardinalExceptionGrouping: false,
			},
			result: SpanV3{
				ErrorEvents: []ErrorEvent{
					{
						Event: Event{
							Name:         "exception",
							TimeUnixNano: uint64(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).AsTime().UnixNano()),
							AttributeMap: map[string]string{
								"exception.type":       "RuntimeError",
								"exception.message":    "Something went wrong",
								"exception.stacktrace": "at line 42\nat line 43",
							},
							IsError: true,
						},
						ErrorGroupID: "092cbbd898be10d4d3d1843203b177cb",
					},
				},
			},
		},
		{
			name: "test_multiple_exception",
			args: args{
				events: func() ptrace.SpanEventSlice {
					events := ptrace.NewSpanEventSlice()
					event := events.AppendEmpty()
					event.SetName("exception")
					event.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
					attrs := event.Attributes()
					attrs.PutStr("exception.type", "RuntimeError")

					event1 := events.AppendEmpty()
					event1.SetName("exception")
					event1.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
					attrs1 := event1.Attributes()
					attrs1.PutStr("exception.type", "DBError")
					return events
				}(),
				span:                         &SpanV3{},
				lowCardinalExceptionGrouping: false,
			},
			result: SpanV3{
				ErrorEvents: []ErrorEvent{
					{
						Event: Event{
							Name:         "exception",
							TimeUnixNano: uint64(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).AsTime().UnixNano()),
							AttributeMap: map[string]string{
								"exception.type": "RuntimeError",
							},
							IsError: true,
						},
						ErrorGroupID: "a334b8fdd25f8fb3e632228494604ee1",
					},
					{
						Event: Event{
							Name:         "exception",
							TimeUnixNano: uint64(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).AsTime().UnixNano()),
							AttributeMap: map[string]string{
								"exception.type": "DBError",
							},
							IsError: true,
						},
						ErrorGroupID: "53a46afc505d4ced5e483a9748486656",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			populateEventsV3(tt.args.events, tt.args.span, tt.args.lowCardinalExceptionGrouping)
			// Compare everything except ErrorID which is randomly generated
			for i := range tt.result.ErrorEvents {
				expected := tt.result.ErrorEvents[i]
				actual := tt.args.span.ErrorEvents[i]

				// Clear ErrorIDs before comparison
				expected.ErrorID = ""
				actual.ErrorID = ""

				if !reflect.DeepEqual(expected, actual) {
					t.Errorf("ErrorEvent[%d] mismatch:\nexpected: %+v\ngot: %+v", i, expected, actual)
				}
			}
		})
	}
}

func Test_newStructuredSpanV3(t *testing.T) {
	type args struct {
		bucketStart uint64
		fingerprint string
		otelSpan    ptrace.Span
		ServiceName string
		resource    pcommon.Resource
		config      storageConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *SpanV3
		wantErr bool
	}{
		{
			name: "test_structured_span",
			args: args{
				bucketStart: 0,
				fingerprint: "test_fingerprint",
				otelSpan: func() ptrace.Span {
					span := ptrace.NewSpan()
					span.SetName("test_span")
					attrs := span.Attributes()
					attrs.PutStr("test_key", "test_value")
					attrs.PutStr("http.url", "http://test.com")
					attrs.PutStr("http.method", "GET")
					attrs.PutStr("http.host", "test.com")
					attrs.PutStr("db.name", "test_db")
					attrs.PutStr("db.operation", "test_operation")
					attrs.PutStr("http.status_code", "200")
					span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
					span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
					span.SetKind(ptrace.SpanKindServer)
					span.SetTraceID(pcommon.NewTraceIDEmpty())
					span.SetSpanID(pcommon.NewSpanIDEmpty())
					return span
				}(),
				ServiceName: "test_service",
				resource: func() pcommon.Resource {
					resource := pcommon.NewResource()
					resource.Attributes().PutStr("service.name", "test_service")
					resource.Attributes().PutInt("num", 10)
					v := resource.Attributes().PutEmptyMap("mymap")
					v.PutStr("map_key", "map_val")
					v.PutDouble("map_double", 20.5)
					return resource
				}(),
				config: storageConfig{},
			},
			want: &SpanV3{
				TsBucketStart:     0,
				FingerPrint:       "test_fingerprint",
				StartTimeUnixNano: uint64(pcommon.NewTimestampFromTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).AsTime().UnixNano()),
				DurationNano:      0,
				Name:              "test_span",
				Kind:              2,
				SpanKind:          "Server",
				StatusCodeString:  "Unset",
				AttributeString: map[string]string{
					"db.name":          "test_db",
					"db.operation":     "test_operation",
					"http.host":        "test.com",
					"http.method":      "GET",
					"http.status_code": "200",
					"http.url":         "http://test.com",
					"test_key":         "test_value",
				},
				AttributesNumber: map[string]float64{},
				AttributesBool:   map[string]bool{},
				ResourcesString: map[string]string{
					"mymap.map_double": "20.5",
					"mymap.map_key":    "map_val",
					"service.name":     "test_service",
					"num":              "10",
				},

				HttpUrl:            "http://test.com",
				HttpMethod:         "GET",
				HttpHost:           "test.com",
				DBName:             "test_db",
				DBOperation:        "test_operation",
				ResponseStatusCode: "200",

				IsRemote:    "unknown",
				HasError:    false,
				References:  `[{"refType":"CHILD_OF"}]`,
				ServiceName: "test_service",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newStructuredSpanV3(tt.args.bucketStart, tt.args.fingerprint, tt.args.otelSpan, tt.args.ServiceName, tt.args.resource, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("newStructuredSpanV3() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got.Tenant = nil
			got.SpanAttributes = []SpanAttribute{}
			if got.TsBucketStart != tt.want.TsBucketStart ||
				got.FingerPrint != tt.want.FingerPrint ||
				got.StartTimeUnixNano != tt.want.StartTimeUnixNano ||
				got.Name != tt.want.Name ||
				got.Kind != tt.want.Kind ||
				got.SpanKind != tt.want.SpanKind ||
				!reflect.DeepEqual(got.AttributeString, tt.want.AttributeString) ||
				!reflect.DeepEqual(got.AttributesNumber, tt.want.AttributesNumber) ||
				!reflect.DeepEqual(got.AttributesBool, tt.want.AttributesBool) ||
				!reflect.DeepEqual(got.ResourcesString, tt.want.ResourcesString) ||
				got.ServiceName != tt.want.ServiceName ||
				got.HttpUrl != tt.want.HttpUrl ||
				got.HttpMethod != tt.want.HttpMethod ||
				got.HttpHost != tt.want.HttpHost ||
				got.DBName != tt.want.DBName ||
				got.DBOperation != tt.want.DBOperation ||
				got.ResponseStatusCode != tt.want.ResponseStatusCode ||
				got.IsRemote != tt.want.IsRemote ||
				got.HasError != tt.want.HasError ||
				got.References != tt.want.References {
				t.Errorf("newStructuredSpanV3() mismatch:\ngot = %+v\nwant = %+v", got, tt.want)
			}
		})
	}
}
