package payloadmeter

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.uber.org/zap"
)

type ProtoSizer struct {
	Logger *zap.Logger
}

func NewProtoSizer(logger *zap.Logger) *ProtoSizer {
	return &ProtoSizer{
		Logger: logger,
	}
}

func (sizer *ProtoSizer) GetSize(data any) int {
	switch v := data.(type) {
	case ptrace.Traces:
		// Calculate size for traces
		return sizer.calculateTracesSize(v)
	case pmetric.Metrics:
		// Calculate size for metrics
		return sizer.calculateMetricsSize(v)
	case plog.Logs:
		// Calculate size for logs
		return sizer.calculateLogsSize(v)
	case map[string]any:
		// Calculate size for map[string]any
		return sizer.calculateMapStringAnySize(v)
	default:
		sizer.Logger.Error("unknown type, setting size to 0", zap.Any("obj", v))
		return 0 // Or handle unexpected type
	}
}

func (sizer *ProtoSizer) calculateTracesSize(traces ptrace.Traces) int {
	return 0
}

func (sizer *ProtoSizer) calculateMetricsSize(metrics pmetric.Metrics) int {
	return 0
}

func (sizer *ProtoSizer) calculateLogsSize(logs plog.Logs) int {
	return 0
}

func (sizer *ProtoSizer) calculateMapStringAnySize(raw map[string]any) int {
	return 0
}
