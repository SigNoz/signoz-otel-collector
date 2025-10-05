// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"fmt"

	"go.uber.org/zap/zapcore"
)

// TODO: Read from github.com/SigNoz/signoz-otel-collector/pkg/schema/traces
type Event struct {
	Name         string            `json:"name,omitempty"`
	TimeUnixNano uint64            `json:"timeUnixNano,omitempty"`
	AttributeMap map[string]string `json:"attributeMap,omitempty"`
	IsError      bool              `json:"isError,omitempty"`
}

func (e *Event) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", e.Name)
	enc.AddUint64("timeUnixNano", e.TimeUnixNano)
	enc.AddBool("isError", e.IsError)
	enc.AddString("attributeMap", fmt.Sprintf("%v", e.AttributeMap))
	return nil
}

type TraceModel struct {
	TraceId           string             `json:"traceId,omitempty"`
	SpanId            string             `json:"spanId,omitempty"`
	Name              string             `json:"name,omitempty"`
	DurationNano      uint64             `json:"durationNano,omitempty"`
	StartTimeUnixNano uint64             `json:"startTimeUnixNano,omitempty"`
	ServiceName       string             `json:"serviceName,omitempty"`
	Kind              int8               `json:"kind,omitempty"`
	SpanKind          string             `json:"spanKind,omitempty"`
	References        references         `json:"references,omitempty"`
	StatusCode        int16              `json:"statusCode,omitempty"`
	TagMap            map[string]string  `json:"tagMap,omitempty"`
	StringTagMap      map[string]string  `json:"stringTagMap,omitempty"`
	NumberTagMap      map[string]float64 `json:"numberTagMap,omitempty"`
	BoolTagMap        map[string]bool    `json:"boolTagMap,omitempty"`
	Events            []string           `json:"event,omitempty"`
	HasError          bool               `json:"hasError,omitempty"`
	StatusMessage     string             `json:"statusMessage,omitempty"`
	StatusCodeString  string             `json:"statusCodeString,omitempty"`
}

func (t *TraceModel) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("traceId", t.TraceId)
	enc.AddString("spanId", t.SpanId)
	enc.AddString("name", t.Name)
	enc.AddUint64("durationNano", t.DurationNano)
	enc.AddUint64("startTimeUnixNano", t.StartTimeUnixNano)
	enc.AddString("serviceName", t.ServiceName)
	enc.AddInt8("kind", t.Kind)
	enc.AddString("spanKind", t.SpanKind)
	enc.AddInt16("statusCode", t.StatusCode)
	enc.AddBool("hasError", t.HasError)
	enc.AddString("statusMessage", t.StatusMessage)
	enc.AddString("statusCodeString", t.StatusCodeString)
	_ = enc.AddArray("references", &t.References)
	enc.AddString("tagMap", fmt.Sprintf("%v", t.TagMap))
	enc.AddString("event", fmt.Sprintf("%v", t.Events))
	return nil
}

type references []OtelSpanRef

func (s *references) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, e := range *s {
		err := enc.AppendObject(&e)
		if err != nil {
			return err
		}
	}
	return nil
}

type Span struct {
	TraceId            string             `json:"traceId,omitempty"`
	SpanId             string             `json:"spanId,omitempty"`
	ParentSpanId       string             `json:"parentSpanId,omitempty"`
	Name               string             `json:"name,omitempty"`
	DurationNano       uint64             `json:"durationNano,omitempty"`
	StartTimeUnixNano  uint64             `json:"startTimeUnixNano,omitempty"`
	ServiceName        string             `json:"serviceName,omitempty"`
	Kind               int8               `json:"kind,omitempty"`
	SpanKind           string             `json:"spanKind,omitempty"`
	StatusCode         int16              `json:"statusCode,omitempty"`
	ExternalHttpMethod string             `json:"externalHttpMethod,omitempty"`
	HttpUrl            string             `json:"httpUrl,omitempty"`
	HttpMethod         string             `json:"httpMethod,omitempty"`
	HttpHost           string             `json:"httpHost,omitempty"`
	HttpRoute          string             `json:"httpRoute,omitempty"`
	MsgSystem          string             `json:"msgSystem,omitempty"`
	MsgOperation       string             `json:"msgOperation,omitempty"`
	ExternalHttpUrl    string             `json:"externalHttpUrl,omitempty"`
	DBSystem           string             `json:"dbSystem,omitempty"`
	DBName             string             `json:"dbName,omitempty"`
	DBOperation        string             `json:"dbOperation,omitempty"`
	PeerService        string             `json:"peerService,omitempty"`
	Events             []string           `json:"event,omitempty"`
	ErrorEvent         Event              `json:"errorEvent,omitempty"`
	ErrorID            string             `json:"errorID,omitempty"`
	ErrorGroupID       string             `json:"errorGroupID,omitempty"`
	StringTagMap       map[string]string  `json:"stringTagMap,omitempty"`
	NumberTagMap       map[string]float64 `json:"numberTagMap,omitempty"`
	BoolTagMap         map[string]bool    `json:"boolTagMap,omitempty"`
	ResourceTagsMap    map[string]string  `json:"resourceTagsMap,omitempty"`
	HasError           bool               `json:"hasError,omitempty"`
	StatusMessage      string             `json:"statusMessage,omitempty"`
	StatusCodeString   string             `json:"statusCodeString,omitempty"`
	IsRemote           string             `json:"isRemote,omitempty"`
	TraceModel         TraceModel         `json:"traceModel,omitempty"`
	RPCSystem          string             `json:"rpcSystem,omitempty"`
	RPCService         string             `json:"rpcService,omitempty"`
	RPCMethod          string             `json:"rpcMethod,omitempty"`
	ResponseStatusCode string             `json:"responseStatusCode,omitempty"`
	Tenant             *string            `json:"-"`
	SpanAttributes     []SpanAttribute    `json:"spanAttributes,omitempty"`
}

// TODO: Read from github.com/SigNoz/signoz-otel-collector/pkg/schema/traces
type ErrorEvent struct {
	Event        Event  `json:"errorEvent,omitempty"`
	ErrorID      string `json:"errorID,omitempty"`
	ErrorGroupID string `json:"errorGroupID,omitempty"`
}

type SpanV3 struct {
	TsBucketStart uint64 `json:"-"`
	FingerPrint   string `json:"-"`

	StartTimeUnixNano uint64 `json:"startTimeUnixNano,omitempty"`

	TraceId      string `json:"traceId,omitempty"`
	SpanId       string `json:"spanId,omitempty"`
	TraceState   string `json:"traceState,omitempty"`
	ParentSpanId string `json:"parentSpanId,omitempty"`
	Flags        uint32 `json:"flags,omitempty"`

	Name string `json:"name,omitempty"`

	Kind     int8   `json:"kind,omitempty"`
	SpanKind string `json:"spanKind,omitempty"`

	DurationNano uint64 `json:"-"`

	StatusCode       int16  `json:"-"`
	StatusMessage    string `json:"-"`
	StatusCodeString string `json:"-"`

	AttributeString  map[string]string  `json:"attributes_string,omitempty"`
	AttributesNumber map[string]float64 `json:"attributes_number,omitempty"`
	AttributesBool   map[string]bool    `json:"attributes_bool,omitempty"`

	ResourcesString map[string]string `json:"-"`
	// billable resource contains filtered keys from resources string which needs to be billed
	// It is using same key as resources_string to keep the billing calculation unchanged
	BillableResourcesString map[string]string `json:"resources_string,omitempty"`

	// for events
	// TODO: Read from github.com/SigNoz/signoz-otel-collector/pkg/schema/traces
	Events []string `json:"event,omitempty"`
	// TODO: Read from github.com/SigNoz/signoz-otel-collector/pkg/schema/traces
	ErrorEvents []ErrorEvent `json:"-"`

	ServiceName string `json:"serviceName,omitempty"` // for error table

	// custom columns
	ResponseStatusCode string `json:"-"`
	ExternalHttpUrl    string `json:"-"`
	HttpUrl            string `json:"-"`
	ExternalHttpMethod string `json:"-"`
	HttpMethod         string `json:"-"`
	HttpHost           string `json:"-"`
	DBName             string `json:"-"`
	DBOperation        string `json:"-"`
	HasError           bool   `json:"-"`
	IsRemote           string `json:"-"`

	// check if this is really required
	Tenant *string `json:"-"`

	References     string          `json:"references,omitempty"`
	SpanAttributes []SpanAttribute `json:"-"`
}

type SpanAttribute struct {
	Key         string
	TagType     string
	DataType    string
	StringValue string
	NumberValue float64
	IsColumn    bool
}

func (s *Span) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("traceId", s.TraceId)
	enc.AddString("spanId", s.SpanId)
	enc.AddString("parentSpanId", s.ParentSpanId)
	enc.AddString("name", s.Name)
	enc.AddUint64("durationNano", s.DurationNano)
	enc.AddUint64("startTimeUnixNano", s.StartTimeUnixNano)
	enc.AddString("serviceName", s.ServiceName)
	enc.AddInt8("kind", s.Kind)
	enc.AddString("spanKind", s.SpanKind)
	enc.AddInt16("statusCode", s.StatusCode)
	enc.AddString("externalHttpMethod", s.ExternalHttpMethod)
	enc.AddString("httpUrl", s.HttpUrl)
	enc.AddString("httpMethod", s.HttpMethod)
	enc.AddString("httpHost", s.HttpHost)
	enc.AddString("httpRoute", s.HttpRoute)
	enc.AddString("msgSystem", s.MsgSystem)
	enc.AddString("msgOperation", s.MsgOperation)
	enc.AddString("externalHttpUrl", s.ExternalHttpUrl)
	enc.AddString("dbSystem", s.DBSystem)
	enc.AddString("dbName", s.DBName)
	enc.AddString("dbOperation", s.DBOperation)
	enc.AddString("peerService", s.PeerService)
	enc.AddString("rpcSystem", s.RPCSystem)
	enc.AddString("rpcService", s.RPCService)
	enc.AddString("rpcMethod", s.RPCMethod)
	enc.AddString("responseStatusCode", s.ResponseStatusCode)
	enc.AddBool("hasError", s.HasError)
	enc.AddString("statusMessage", s.StatusMessage)
	enc.AddString("statusCodeString", s.StatusCodeString)
	enc.AddString("errorID", s.ErrorID)
	enc.AddString("errorGroupID", s.ErrorGroupID)
	_ = enc.AddObject("errorEvent", &s.ErrorEvent)
	_ = enc.AddObject("traceModel", &s.TraceModel)
	enc.AddString("event", fmt.Sprintf("%v", s.Events))

	return nil
}

// TODO: Read from github.com/SigNoz/signoz-otel-collector/pkg/schema/traces
type OtelSpanRef struct {
	TraceId string `json:"traceId,omitempty"`
	SpanId  string `json:"spanId,omitempty"`
	RefType string `json:"refType,omitempty"`
}

func (r *OtelSpanRef) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("traceId", r.TraceId)
	enc.AddString("spanId", r.SpanId)
	enc.AddString("refType", r.RefType)
	return nil
}
