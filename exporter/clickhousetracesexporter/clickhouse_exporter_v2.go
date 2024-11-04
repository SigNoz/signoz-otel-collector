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
	"time"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/google/uuid"
	"github.com/segmentio/ksuid"
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
		} else if k == "http.route" {
			span.HttpRoute = v.Str()
		} else if k == "http.host" || k == "server.address" ||
			k == "client.address" || k == "http.request.header.host" {
			span.HttpHost = v.Str()
		} else if k == "messaging.system" {
			span.MsgSystem = v.Str()
		} else if k == "messaging.operation" {
			span.MsgOperation = v.Str()
		} else if k == "db.system" {
			span.DBSystem = v.Str()
		} else if k == "db.name" || k == "db.namespace" {
			span.DBName = v.Str()
		} else if k == "db.operation" || k == "db.operation.name" {
			span.DBOperation = v.Str()
		} else if k == "peer.service" {
			span.PeerService = v.Str()
		} else if k == "rpc.grpc.status_code" {
			// Handle both string/int status code in GRPC spans.
			statusString, err := strconv.Atoi(v.Str())
			statusInt := v.Int()
			if err == nil && statusString != 0 {
				statusInt = int64(statusString)
			}
			span.ResponseStatusCode = strconv.FormatInt(statusInt, 10)
		} else if k == "rpc.method" {
			span.RPCMethod = v.Str()
		} else if k == "rpc.service" {
			span.RPCService = v.Str()
		} else if k == "rpc.system" {
			span.RPCSystem = v.Str()
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
		if event.Name == "exception" {
			event.IsError = true
			span.ErrorEvent = event
			uuidWithHyphen := uuid.New()
			uuid := strings.Replace(uuidWithHyphen.String(), "-", "", -1)
			span.ErrorID = uuid
			var hash [16]byte
			if lowCardinalExceptionGrouping {
				hash = md5.Sum([]byte(span.ServiceName + span.ErrorEvent.AttributeMap["exception.type"]))
			} else {
				hash = md5.Sum([]byte(span.ServiceName + span.ErrorEvent.AttributeMap["exception.type"] + span.ErrorEvent.AttributeMap["exception.message"]))

			}
			span.ErrorGroupID = fmt.Sprintf("%x", hash)
		}
		stringEvent, _ := json.Marshal(event)
		span.Events = append(span.Events, string(stringEvent))
	}
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

	attributes := otelSpan.Attributes()
	resourceAttributes := resource.Attributes()
	tagMap := map[string]string{}
	stringTagMap := map[string]string{}
	numberTagMap := map[string]float64{}
	boolTagMap := map[string]bool{}
	spanAttributes := []SpanAttribute{}

	resourceAttrs := map[string]string{}

	attributes.Range(func(k string, v pcommon.Value) bool {
		tagMap[k] = v.AsString()
		spanAttribute := SpanAttribute{
			Key:      k,
			TagType:  "tag",
			IsColumn: false,
		}
		if v.Type() == pcommon.ValueTypeDouble {
			numberTagMap[k] = v.Double()
			spanAttribute.NumberValue = v.Double()
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeInt {
			numberTagMap[k] = float64(v.Int())
			spanAttribute.NumberValue = float64(v.Int())
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeBool {
			boolTagMap[k] = v.Bool()
			spanAttribute.DataType = "bool"
		} else {
			stringTagMap[k] = v.AsString()
			spanAttribute.StringValue = v.AsString()
			spanAttribute.DataType = "string"
		}
		spanAttributes = append(spanAttributes, spanAttribute)
		return true

	})

	resourceAttributes.Range(func(k string, v pcommon.Value) bool {
		tagMap[k] = v.AsString()
		spanAttribute := SpanAttribute{
			Key:      k,
			TagType:  "resource",
			IsColumn: false,
		}
		resourceAttrs[k] = v.AsString()
		if v.Type() == pcommon.ValueTypeDouble {
			numberTagMap[k] = v.Double()
			spanAttribute.NumberValue = v.Double()
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeInt {
			numberTagMap[k] = float64(v.Int())
			spanAttribute.NumberValue = float64(v.Int())
			spanAttribute.DataType = "float64"
		} else if v.Type() == pcommon.ValueTypeBool {
			boolTagMap[k] = v.Bool()
			spanAttribute.DataType = "bool"
		} else {
			stringTagMap[k] = v.AsString()
			spanAttribute.StringValue = v.AsString()
			spanAttribute.DataType = "string"
		}
		spanAttributes = append(spanAttributes, spanAttribute)
		return true

	})

	references, _ := makeJaegerProtoReferences(otelSpan.Links(), otelSpan.ParentSpanID(), otelSpan.TraceID())
	referencesBytes, _ := json.Marshal(references)

	tenant := usage.GetTenantNameFromResource(resource)

	// generate the id from timestamp
	id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(otelSpan.StartTimestamp())))
	if err != nil {
		return nil, fmt.Errorf("IdGenError:%w", err)
	}

	var span *SpanV2 = &SpanV2{
		TsBucketStart: bucketStart,
		FingerPrint:   fingerprint,

		StartTimeUnixNano: uint64(otelSpan.StartTimestamp()),
		Id:                id.String(),

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

		AttributeString:  stringTagMap,
		AttributesNumber: numberTagMap,
		AttributesBool:   boolTagMap,

		ResourcesString: resourceAttrs,

		ServiceName: ServiceName,

		IsRemote: isRemote,

		Tenant: &tenant,

		References: string(referencesBytes),
	}

	if otelSpan.Status().Code() == ptrace.StatusCodeError {
		span.HasError = true
	}

	populateCustomAttrsAndAttrs(attributes, span)

	populateEventsV2(otelSpan.Events(), span, config.lowCardinalExceptionGrouping)
	spanAttributes = append(spanAttributes, extractSpanAttributesFromSpanIndexV2(span)...)
	span.SpanAttributes = spanAttributes
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

func extractSpanAttributesFromSpanIndexV2(span *SpanV2) []SpanAttribute {
	spanAttributes := []SpanAttribute{}
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "traceID",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.TraceId,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "spanID",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.SpanId,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "parentSpanID",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.ParentSpanId,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "name",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.Name,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "serviceName",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.ServiceName,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "kind",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "float64",
		NumberValue: float64(span.Kind),
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "spanKind",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.SpanKind,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "durationNano",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "float64",
		NumberValue: float64(span.DurationNano),
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "statusCode",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "float64",
		NumberValue: float64(span.StatusCode),
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:      "hasError",
		TagType:  "tag",
		IsColumn: true,
		DataType: "bool",
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "statusMessage",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.StatusMessage,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "statusCodeString",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.StatusCodeString,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "externalHttpMethod",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.ExternalHttpMethod,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "externalHttpUrl",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.ExternalHttpUrl,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "dbSystem",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.DBSystem,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "dbName",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.DBName,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "dbOperation",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.DBOperation,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "peerService",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.PeerService,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "httpMethod",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.HttpMethod,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "httpUrl",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.HttpUrl,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "httpRoute",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.HttpRoute,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "httpHost",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.HttpHost,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "msgSystem",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.MsgSystem,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "msgOperation",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.MsgOperation,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "rpcSystem",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.RPCSystem,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "rpcService",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.RPCService,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "rpcMethod",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.RPCMethod,
	})
	spanAttributes = append(spanAttributes, SpanAttribute{
		Key:         "responseStatusCode",
		TagType:     "tag",
		IsColumn:    true,
		DataType:    "string",
		StringValue: span.ResponseStatusCode,
	})
	return spanAttributes
}
