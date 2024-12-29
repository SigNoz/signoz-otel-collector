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
		return sizer.calculateTracesSize(v)
	case pmetric.Metrics:
		return sizer.calculateMetricsSize(v)
	case plog.Logs:
		return sizer.calculateLogsSize(v)
	case map[string]any:
		return sizer.calculateMapStringAnySize(v)
	default:
		sizer.Logger.Error("unknown type, setting size to 0", zap.Any("obj", v))
		return 0
	}
}

func (sizer *ProtoSizer) calculateTracesSize(traces ptrace.Traces) int {
	// todo implement it for proto
	return 0
}

func (sizer *ProtoSizer) calculateMetricsSize(metrics pmetric.Metrics) int {
	// todo implement it for proto
	return 0
}

func (sizer *ProtoSizer) calculateLogsSize(logs plog.Logs) int {
	// todo implement it for proto
	return 0
}

func (sizer *ProtoSizer) calculateMapStringAnySize(raw map[string]any) int {
	// todo implement it for proto
	return 0
}
