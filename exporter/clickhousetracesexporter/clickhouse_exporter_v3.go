package clickhousetracesexporter

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
)

func makeJaegerProtoReferences(
	links ptrace.SpanLinkSlice,
	parentSpanID pcommon.SpanID,
	traceID pcommon.TraceID,
) ([]OtelSpanRef, error) {

	parentSpanIDSet := len([8]byte(parentSpanID)) != 0
	if !parentSpanIDSet && links.Len() == 0 {
		return nil, nil
	}

	refsCount := links.Len()
	if parentSpanIDSet {
		refsCount++
	}

	refs := make([]OtelSpanRef, 0, refsCount)

	// Put parent span ID at the first place because usually backends look for it
	// as the first CHILD_OF item in the model.SpanRef slice.
	if parentSpanIDSet {

		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(traceID),
			SpanId:  utils.SpanIDToHexOrEmptyString(parentSpanID),
			RefType: "CHILD_OF",
		})
	}

	for i := 0; i < links.Len(); i++ {
		link := links.At(i)

		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(link.TraceID()),
			SpanId:  utils.SpanIDToHexOrEmptyString(link.SpanID()),

			// Since Jaeger RefType is not captured in internal data,
			// use SpanRefType_FOLLOWS_FROM by default.
			// SpanRefType_CHILD_OF supposed to be set only from parentSpanID.
			RefType: "FOLLOWS_FROM",
		})
	}

	return refs, nil
}

// ServiceNameForResource gets the service name for a specified Resource.
// TODO: Find a better package for this function.
func ServiceNameForResource(resource pcommon.Resource) string {

	service, found := resource.Attributes().Get(string(conventions.ServiceNameKey))
	if !found {
		return "<nil-service-name>"
	}

	return service.Str()
}

func populateCustomAttrsAndAttrs(attributes pcommon.Map, span *SpanV3) {
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

func populateEventsV3(events ptrace.SpanEventSlice, span *SpanV3, lowCardinalExceptionGrouping bool) {
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
		if utils.IsValidFloat(value.Double()) {
			attrMap.NumberMap[key] = value.Double()
			spanAttribute.NumberValue = value.Double()
			spanAttribute.DataType = "float64"
		} else {
			zap.S().Warn("NaN value in tag map, skipping key: ", zap.String("key", key))
			return
		}
	} else if value.Type() == pcommon.ValueTypeInt {
		attrMap.NumberMap[key] = float64(value.Int())
		spanAttribute.NumberValue = float64(value.Int())
		spanAttribute.DataType = "float64"
	} else if value.Type() == pcommon.ValueTypeBool {
		attrMap.BoolMap[key] = value.Bool()
		spanAttribute.DataType = "bool"
	} else if value.Type() == pcommon.ValueTypeMap {
		// flatten map
		result := flatten.FlattenJSON(value.Map().AsRaw(), key)
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
				if utils.IsValidFloat(tempVal) {
					attrMap.NumberMap[tempKey] = tempVal
					tSpanAttribute.NumberValue = tempVal
					tSpanAttribute.DataType = "float64"
				} else {
					zap.S().Warn("NaN value in tag map, skipping key: ", zap.String("key", tempKey))
					continue
				}
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

func newStructuredSpanV3(bucketStart uint64, fingerprint string, otelSpan ptrace.Span, ServiceName string, resource pcommon.Resource, config storageConfig) (*SpanV3, error) {
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
	billableResourceAttrs := map[string]string{}

	otelSpan.Attributes().Range(func(k string, v pcommon.Value) bool {
		attrMap.add(k, v)
		return true

	})

	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		isBillable := !metering.ExcludeSigNozWorkspaceResourceAttrs.MatchString(k)
		if v.Type() == pcommon.ValueTypeMap {
			result := flatten.FlattenJSON(v.Map().AsRaw(), k)
			for tempKey, tempVal := range result {
				strVal := fmt.Sprintf("%v", tempVal)
				resourceAttrs[tempKey] = strVal
				if isBillable {
					billableResourceAttrs[tempKey] = strVal
				}
				attrMap.SpanAttributes = append(attrMap.SpanAttributes, SpanAttribute{
					Key:         tempKey,
					TagType:     "resource",
					IsColumn:    false,
					StringValue: strVal,
					DataType:    "string",
				})
			}
		} else {
			resourceAttrs[k] = v.AsString()
			if isBillable {
				billableResourceAttrs[k] = v.AsString()
			}
			attrMap.SpanAttributes = append(attrMap.SpanAttributes, SpanAttribute{
				Key:         k,
				TagType:     "resource",
				IsColumn:    false,
				StringValue: v.AsString(),
				DataType:    "string",
			})
		}
		return true

	})

	references, _ := makeJaegerProtoReferences(otelSpan.Links(), otelSpan.ParentSpanID(), otelSpan.TraceID())
	referencesBytes, _ := json.Marshal(references)

	tenant := usage.GetTenantNameFromResource()

	var span *SpanV3 = &SpanV3{
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

		ResourcesString:         resourceAttrs,
		BillableResourcesString: billableResourceAttrs,

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
	populateEventsV3(otelSpan.Events(), span, config.lowCardinalExceptionGrouping)
	return span, nil
}

// traceDataPusher implements OTEL exporterhelper.traceDataPusher

func tsBucket(ts int64, bucketSize int64) int64 {
	return (int64(ts) / int64(bucketSize)) * int64(bucketSize)
}

func (s *clickhouseTracesExporter) pushTraceDataV3(ctx context.Context, td ptrace.Traces) error {
	s.wg.Add(1)
	defer s.wg.Done()

	oldestAllowedTs := time.Now().Add(-time.Duration(s.maxAllowedDataAgeDays) * 24 * time.Hour).UnixNano()

	resourcesSeen := map[int64]map[string]string{}

	select {
	case <-s.closeChan:
		return errors.New("shutdown has been called")
	default:
		rss := td.ResourceSpans()
		var batchOfSpans []*SpanV3

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
				return fmt.Errorf("couldn't serialize trace resource JSON: %w", err)
			}
			resourceJson := string(serializedRes)

			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ils := rs.ScopeSpans().At(j)

				spans := ils.Spans()

				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)
					ts := uint64(span.StartTimestamp())
					if ts < uint64(oldestAllowedTs) {
						s.logger.Debug(
							"skipping span outside allowed time window",
							zap.Uint64("start_ts", ts),
							zap.Int64("oldest_allowed_ts", oldestAllowedTs),
							zap.String("service_name", serviceName),
							zap.String("span_name", span.Name()),
							zap.String("trace_id", utils.TraceIDToHexOrEmptyString(span.TraceID())),
							zap.String("span_id", utils.SpanIDToHexOrEmptyString(span.SpanID())),
							zap.String("parent_span_id", utils.SpanIDToHexOrEmptyString(span.ParentSpanID())),
							zap.String("start_time", span.StartTimestamp().AsTime().Format(time.RFC3339)),
							zap.String("end_time", span.EndTimestamp().AsTime().Format(time.RFC3339)),
							zap.String("span_kind", span.Kind().String()),
							zap.String("status_code", span.Status().Code().String()),
							zap.String("status_message", span.Status().Message()),
						)
						continue
					}

					lBucketStart := tsBucket(int64(span.StartTimestamp()/1000000000), distributedTracesResourceV2Seconds)
					if _, exists := resourcesSeen[int64(lBucketStart)]; !exists {
						resourcesSeen[int64(lBucketStart)] = map[string]string{}
					}
					fp, exists := resourcesSeen[int64(lBucketStart)][resourceJson]
					if !exists {
						fp = fingerprint.CalculateFingerprint(rs.Resource().Attributes().AsRaw(), fingerprint.ResourceHierarchy())
						resourcesSeen[int64(lBucketStart)][resourceJson] = fp
					}

					structuredSpan, err := newStructuredSpanV3(uint64(lBucketStart), fp, span, serviceName, rs.Resource(), s.config)
					if err != nil {
						return fmt.Errorf("failed to create newStructuredSpanV3: %w", err)
					}
					batchOfSpans = append(batchOfSpans, structuredSpan)

					serializedStructuredSpan, err := json.Marshal(structuredSpan)
					if err != nil {
						return fmt.Errorf("failed to marshal structured span: %w", err)
					}
					size += len(serializedStructuredSpan)
					count += 1
				}
			}
		}

		usage.AddMetric(metrics, "default", int64(count), int64(size))

		err := s.Writer.WriteBatchOfSpansV3(ctx, batchOfSpans, metrics)
		if err != nil {
			return fmt.Errorf("error in writing spans to clickhouse: %w", err)
		}

		// write the resources
		err = s.Writer.WriteResourcesV3(ctx, resourcesSeen)
		if err != nil {
			return fmt.Errorf("error in writing resources to clickhouse: %w", err)
		}

		return nil
	}
}
