package metering

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Meter is an interface that receives telemetry data and
// calculates billable metrics
type Meter[T ptrace.Traces | pmetric.Metrics | plog.Logs] interface {
	Size(T) int
	Count(T) int
}

type Logs interface {
	Meter[plog.Logs]
}

type Sizer interface {
	SizeOfMapStringAny(map[string]any) int
}
