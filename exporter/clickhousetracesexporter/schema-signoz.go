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

type Event struct {
	Name         string            `json:"name,omitempty"`
	TimeUnixNano uint64            `json:"timeUnixNano,omitempty"`
	AttributeMap map[string]string `json:"attributeMap,omitempty"`
	IsError      bool              `json:"isError,omitempty"`
}

type TraceModel struct {
	TraceId           string            `json:"traceId,omitempty"`
	SpanId            string            `json:"spanId,omitempty"`
	Name              string            `json:"name,omitempty"`
	DurationNano      uint64            `json:"durationNano,omitempty"`
	StartTimeUnixNano uint64            `json:"startTimeUnixNano,omitempty"`
	ServiceName       string            `json:"serviceName,omitempty"`
	Kind              int8              `json:"kind,omitempty"`
	References        []OtelSpanRef     `json:"references,omitempty"`
	StatusCode        int16             `json:"statusCode,omitempty"`
	TagMap            map[string]string `json:"tagMap,omitempty"`
	Events            []string          `json:"event,omitempty"`
	HasError          bool              `json:"hasError,omitempty"`
}

type Span struct {
	TraceId            string            `json:"traceId,omitempty"`
	SpanId             string            `json:"spanId,omitempty"`
	ParentSpanId       string            `json:"parentSpanId,omitempty"`
	Name               string            `json:"name,omitempty"`
	DurationNano       uint64            `json:"durationNano,omitempty"`
	StartTimeUnixNano  uint64            `json:"startTimeUnixNano,omitempty"`
	ServiceName        string            `json:"serviceName,omitempty"`
	Kind               int8              `json:"kind,omitempty"`
	StatusCode         int16             `json:"statusCode,omitempty"`
	ExternalHttpMethod string            `json:"externalHttpMethod,omitempty"`
	HttpUrl            string            `json:"httpUrl,omitempty"`
	HttpMethod         string            `json:"httpMethod,omitempty"`
	HttpHost           string            `json:"httpHost,omitempty"`
	HttpRoute          string            `json:"httpRoute,omitempty"`
	HttpCode           string            `json:"httpCode,omitempty"`
	MsgSystem          string            `json:"msgSystem,omitempty"`
	MsgOperation       string            `json:"msgOperation,omitempty"`
	ExternalHttpUrl    string            `json:"externalHttpUrl,omitempty"`
	Component          string            `json:"component,omitempty"`
	DBSystem           string            `json:"dbSystem,omitempty"`
	DBName             string            `json:"dbName,omitempty"`
	DBOperation        string            `json:"dbOperation,omitempty"`
	PeerService        string            `json:"peerService,omitempty"`
	Events             []string          `json:"event,omitempty"`
	ErrorEvent         Event             `json:"errorEvent,omitempty"`
	ErrorID            string            `json:"errorID,omitempty"`
	ErrorGroupID       string            `json:"errorGroupID,omitempty"`
	TagMap             map[string]string `json:"tagMap,omitempty"`
	HasError           bool              `json:"hasError,omitempty"`
	TraceModel         TraceModel        `json:"traceModel,omitempty"`
	GRPCCode           string            `json:"gRPCCode,omitempty"`
	GRPCMethod         string            `json:"gRPCMethod,omitempty"`
	RPCSystem          string            `json:"rpcSystem,omitempty"`
	RPCService         string            `json:"rpcService,omitempty"`
	RPCMethod          string            `json:"rpcMethod,omitempty"`
	ResponseStatusCode string            `json:"responseStatusCode,omitempty"`
}

type OtelSpanRef struct {
	TraceId string `json:"traceId,omitempty"`
	SpanId  string `json:"spanId,omitempty"`
	RefType string `json:"refType,omitempty"`
}
