package clickhousetracesexporter

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func populateCustomAttrsAndAttrs(attributes pcommon.Map, span *SpanV2) {

	attributes.Range(func(k string, v pcommon.Value) bool {
		if k == "http.status_code" || k == "http.response.status_code" {
			// Handle both string/int http status codes.
			statusString, err := strconv.Atoi(v.Str())
			statusInt := v.Int()
			if err == nil && statusString != 0 {
				statusInt = int64(statusString)
			}
			span.ResponseStatusCode = strconv.FormatInt(statusInt, 10)
		} else if (k == "http.url" || k == "url.full") && span.Kind == 3 {
			value := v.Str()
			valueUrl, err := url.Parse(value)
			if err == nil {
				value = valueUrl.Hostname()
			}
			span.ExternalHttpUrl = value
			span.HttpUrl = v.Str()
		} else if (k == "http.method" || k == "http.request.method") && span.Kind == 3 {
			span.ExternalHttpMethod = v.Str()
			span.HttpMethod = v.Str()
		} else if (k == "http.url" || k == "url.full") && span.Kind != 3 {
			span.HttpUrl = v.Str()
		} else if (k == "http.method" || k == "http.request.method") && span.Kind != 3 {
			span.HttpMethod = v.Str()
		} else if k == "http.host" || k == "server.address" ||
			k == "client.address" || k == "http.request.header.host" {
			span.HttpHost = v.Str()
		} else if k == "db.name" || k == "db.namespace" {
			span.DBName = v.Str()
		} else if k == "db.operation" || k == "db.operation.name" {
			span.DBOperation = v.Str()
		} else if k == "rpc.grpc.status_code" {
			// Handle both string/int status code in GRPC spans.
			statusString, err := strconv.Atoi(v.Str())
			statusInt := v.Int()
			if err == nil && statusString != 0 {
				statusInt = int64(statusString)
			}
			span.ResponseStatusCode = strconv.FormatInt(statusInt, 10)
		} else if k == "rpc.jsonrpc.error_code" {
			span.ResponseStatusCode = v.Str()
		}
		return true

	})

}

func populateEventsV2(events ptrace.SpanEventSlice, span *SpanV2, lowCardinalExceptionGrouping bool) {
	for i := 0; i < events.Len(); i++ {
		event := Event{}
		event.Name = events.At(i).Name()
		event.TimeUnixNano = uint64(events.At(i).Timestamp())
		event.AttributeMap = map[string]string{}
		event.IsError = false
		events.At(i).Attributes().Range(func(k string, v pcommon.Value) bool {
			event.AttributeMap[k] = v.AsString()
			return true
		})
		errorEvent := ErrorEvent{}
		if event.Name == "exception" {
			event.IsError = true
			errorEvent.Event = event
			uuidWithHyphen := uuid.New()
			uuid := strings.Replace(uuidWithHyphen.String(), "-", "", -1)
			errorEvent.ErrorID = uuid
			var hash [16]byte
			if lowCardinalExceptionGrouping {
				hash = md5.Sum([]byte(span.ServiceName + errorEvent.Event.AttributeMap["exception.type"]))
			} else {
				hash = md5.Sum([]byte(span.ServiceName + errorEvent.Event.AttributeMap["exception.type"] + errorEvent.Event.AttributeMap["exception.message"]))
			}
			errorEvent.ErrorGroupID = fmt.Sprintf("%x", hash)
		}
		stringEvent, _ := json.Marshal(event)
		span.Events = append(span.Events, string(stringEvent))
		span.ErrorEvents = append(span.ErrorEvents, errorEvent)
	}
}

type attributesData struct {
	StringMap      map[string]string
	NumberMap      map[string]float64
	BoolMap        map[string]bool
	SpanAttributes []SpanAttribute
}

func (attrMap *attributesData) add(key string, value pcommon.Value) {
	spanAttribute := SpanAttribute{
		Key:      key,
		TagType:  "tag",
		IsColumn: false,
	}

	if value.Type() == pcommon.ValueTypeDouble {
		attrMap.NumberMap[key] = value.Double()
		spanAttribute.NumberValue = value.Double()
		spanAttribute.DataType = "float64"
	} else if value.Type() == pcommon.ValueTypeInt {
		attrMap.NumberMap[key] = float64(value.Int())
		spanAttribute.NumberValue = float64(value.Int())
		spanAttribute.DataType = "float64"
	} else if value.Type() == pcommon.ValueTypeBool {
		attrMap.BoolMap[key] = value.Bool()
		spanAttribute.DataType = "bool"
	} else if value.Type() == pcommon.ValueTypeMap {
		// flatten map
		result := flatten.FlattenJSON(value.Map().AsRaw(), "")
		for tempKey, tempVal := range result {
			tSpanAttribute := SpanAttribute{
				Key:      tempKey,
				TagType:  "tag",
				IsColumn: false,
			}
			switch tempVal := tempVal.(type) {
			case string:
				attrMap.StringMap[tempKey] = tempVal
				tSpanAttribute.StringValue = tempVal
				tSpanAttribute.DataType = "string"
			case float64:
				attrMap.NumberMap[tempKey] = tempVal
				tSpanAttribute.NumberValue = tempVal
				tSpanAttribute.DataType = "float64"
			case bool:
				attrMap.BoolMap[tempKey] = tempVal
				tSpanAttribute.DataType = "bool"
			}
			attrMap.SpanAttributes = append(attrMap.SpanAttributes, tSpanAttribute)
		}
		return
	} else {
		attrMap.StringMap[key] = value.AsString()
		spanAttribute.StringValue = value.AsString()
		spanAttribute.DataType = "string"
	}
	attrMap.SpanAttributes = append(attrMap.SpanAttributes, spanAttribute)
}

func newStructuredSpanV2(bucketStart uint64, fingerprint string, otelSpan ptrace.Span, ServiceName string, resource pcommon.Resource, config storageConfig) (*SpanV2, error) {
	durationNano := uint64(otelSpan.EndTimestamp() - otelSpan.StartTimestamp())

	isRemote := "unknown"
	flags := otelSpan.Flags()
	if flags&hasIsRemoteMask != 0 {
		isRemote = "no"
		if flags&isRemoteMask != 0 {
			isRemote = "yes"
		}
	}

	attrMap := attributesData{
		StringMap:      make(map[string]string),
		NumberMap:      make(map[string]float64),
		BoolMap:        make(map[string]bool),
		SpanAttributes: []SpanAttribute{},
	}

	resourceAttrs := map[string]string{}

	otelSpan.Attributes().Range(func(k string, v pcommon.Value) bool {
		attrMap.add(k, v)
		return true

	})

	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		spanAttribute := SpanAttribute{
			Key:      k,
			TagType:  "resource",
			IsColumn: false,
		}
		resourceAttrs[k] = v.AsString()
		if v.Type() == pcommon.ValueTypeDouble {
			spanAttribute.NumberValue = v.Double()
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeInt {
			spanAttribute.NumberValue = float64(v.Int())
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeBool {
			spanAttribute.DataType = "bool"
		} else {
			spanAttribute.StringValue = v.AsString()
			spanAttribute.DataType = "string"
		}
		attrMap.SpanAttributes = append(attrMap.SpanAttributes, spanAttribute)
		return true

	})

	references, _ := makeJaegerProtoReferences(otelSpan.Links(), otelSpan.ParentSpanID(), otelSpan.TraceID())
	referencesBytes, _ := json.Marshal(references)

	tenant := usage.GetTenantNameFromResource(resource)

	var span *SpanV2 = &SpanV2{
		TsBucketStart: bucketStart,
		FingerPrint:   fingerprint,

		StartTimeUnixNano: uint64(otelSpan.StartTimestamp()),

		TraceId:      utils.TraceIDToHexOrEmptyString(otelSpan.TraceID()),
		SpanId:       utils.SpanIDToHexOrEmptyString(otelSpan.SpanID()),
		TraceState:   otelSpan.TraceState().AsRaw(),
		ParentSpanId: utils.SpanIDToHexOrEmptyString(otelSpan.ParentSpanID()),
		Flags:        otelSpan.Flags(),

		Name: otelSpan.Name(),

		Kind:     int8(otelSpan.Kind()),
		SpanKind: otelSpan.Kind().String(),

		DurationNano: durationNano,

		StatusCode:       int16(otelSpan.Status().Code()),
		StatusMessage:    otelSpan.Status().Message(),
		StatusCodeString: otelSpan.Status().Code().String(),

		AttributeString:  attrMap.StringMap,
		AttributesNumber: attrMap.NumberMap,
		AttributesBool:   attrMap.BoolMap,

		ResourcesString: resourceAttrs,

		ServiceName: ServiceName,

		IsRemote: isRemote,

		Tenant: &tenant,

		References:     string(referencesBytes),
		SpanAttributes: attrMap.SpanAttributes,
	}

	if otelSpan.Status().Code() == ptrace.StatusCodeError {
		span.HasError = true
	}

	populateCustomAttrsAndAttrs(otelSpan.Attributes(), span)
	populateEventsV2(otelSpan.Events(), span, config.lowCardinalExceptionGrouping)
	return span, nil
}

// traceDataPusher implements OTEL exporterhelper.traceDataPusher

func tsBucket(ts int64, bucketSize int64) int64 {
	return (int64(ts) / int64(bucketSize)) * int64(bucketSize)
}

const (
	DISTRIBUTED_TRACES_RESOURCE_V2_SECONDS = 1800
)

func (s *storage) pushTraceDataV2(ctx context.Context, td ptrace.Traces) error {
	s.wg.Add(1)
	defer s.wg.Done()

	resourcesSeen := map[int64]map[string]string{}

	select {
	case <-s.closeChan:
		return errors.New("shutdown has been called")
	default:
		rss := td.ResourceSpans()
		var batchOfSpans []*SpanV2

		count := 0
		size := 0
		metrics := map[string]usage.Metric{}
		for i := 0; i < rss.Len(); i++ {
			rs := rss.At(i)

			serviceName := ServiceNameForResource(rs.Resource())

			// convert this to a string
			stringMap := make(map[string]string, len(rs.Resource().Attributes().AsRaw()))
			rs.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
				stringMap[k] = v.AsString()
				return true
			})
			serializedRes, err := json.Marshal(stringMap)
			if err != nil {
				return fmt.Errorf("couldn't serialize log resource JSON: %w", err)
			}
			resourceJson := string(serializedRes)

			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ils := rs.ScopeSpans().At(j)

				spans := ils.Spans()

				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)

					lBucketStart := tsBucket(int64(span.StartTimestamp()/1000000000), DISTRIBUTED_TRACES_RESOURCE_V2_SECONDS)
					if _, exists := resourcesSeen[int64(lBucketStart)]; !exists {
						resourcesSeen[int64(lBucketStart)] = map[string]string{}
					}
					fp, exists := resourcesSeen[int64(lBucketStart)][resourceJson]
					if !exists {
						fp = fingerprint.CalculateFingerprint(rs.Resource().Attributes().AsRaw(), fingerprint.ResourceHierarchy())
						resourcesSeen[int64(lBucketStart)][resourceJson] = fp
					}

					structuredSpan, err := newStructuredSpanV2(uint64(lBucketStart), fp, span, serviceName, rs.Resource(), s.config)
					if err != nil {
						zap.S().Error("Error in creating newStructuredSpanV2: ", err)
						return err
					}
					batchOfSpans = append(batchOfSpans, structuredSpan)

					serializedStructuredSpan, _ := json.Marshal(structuredSpan)
					size += len(serializedStructuredSpan)
					count += 1
				}
			}
		}

		if s.useNewSchema {
			usage.AddMetric(metrics, "default", int64(count), int64(size))
		}

		err := s.Writer.WriteBatchOfSpansV2(ctx, batchOfSpans, metrics)
		if err != nil {
			zap.S().Error("Error in writing spans to clickhouse: ", err)
			return err
		}

		// write the resources
		err = s.Writer.WriteResourcesV2(ctx, resourcesSeen)
		if err != nil {
			zap.S().Error("Error in writing resources to clickhouse: ", err)
			return err
		}

		return nil
	}
}
