package metering

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Meter is an interface that receives telemetry data and
// calculates billable metrics.
type Meter[T ptrace.Traces | pmetric.Metrics | plog.Logs] interface {
	// Size calculates size of the telemetry data in bytes.
	Size(T) int
	// Count calculates count of the telemetry data.
	Count(T) int
}

// Sizer is an interface that calculates the size of different of map[string]any
type Sizer interface {
	SizeOfMapStringAny(map[string]any) int
}

// Logs calculates billable metrics for logs.
type Logs interface {
	Meter[plog.Logs]
}

// Traces calculates billable metrics for traces.
type Traces interface {
	Meter[ptrace.Traces]
}

// Metrics calculates billable metrics for metrics.
type Metrics interface {
	Meter[pmetric.Metrics]
}
