package payloadmeter

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Meter[T ptrace.Traces | pmetric.Metrics | plog.Logs] interface {
	// Calculates size of the telemetry data in bytes.
	Size(T, Decoding) int
	// Calculates count of the telemetry data.
	Count(T, Decoding) int
}

// Sizer is an interface that calculates the size of different
// data structures
type Sizer interface {
	GetSize(data any) int
	//Encoding() Serialization
}

// Calculates billable metrics for logs.
type Logs interface {
	Meter[plog.Logs]
}

// Calculates billable metrics for traces.
type Traces interface {
	Meter[ptrace.Traces]
}

// Calculates billable metrics for metrics.
type Metrics interface {
	Meter[pmetric.Metrics]
}

type Serialization struct {
	name string
}

var (
	Json  = Serialization{name: "json"}
	Proto = Serialization{name: "proto"}
)

type Decoding struct {
	name string
}

var (
	// Otlp how upstream calculation size
	Otlp = Decoding{name: "otlp"}
	// Signoz custom calculation of size for signoz, used in billing, does the deep calculation
	Signoz = Decoding{name: "signoz"}
)
