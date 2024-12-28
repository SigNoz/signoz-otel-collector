package payloadmeter

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Meter is an interface that receives telemetry data and
// calculates billable metrics.
type Meter[T ptrace.Traces | pmetric.Metrics | plog.Logs] interface {
	// Calculates size of the telemetry data in bytes.
	Size(T) int
	// Calculates count of the telemetry data.
	Count(T) int
}

// Sizer is an interface that calculates the size of different
// data structures
type Sizer interface {
	SizeOfMapStringAny(map[string]any) int
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
