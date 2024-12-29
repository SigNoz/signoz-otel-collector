package payloadmeter

import (
	"encoding/json"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.uber.org/zap"
)

type JsonSizer struct {
	Logger *zap.Logger
}

func NewJSONSizer(logger *zap.Logger) *JsonSizer {
	return &JsonSizer{
		Logger: logger,
	}
}

func (sizer *JsonSizer) GetSize(data any) int {
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

func (sizer *JsonSizer) calculateTracesSize(traces ptrace.Traces) int {
	bytes, err := (&ptrace.JSONMarshaler{}).MarshalTraces(traces)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Any("obj", traces))
		return 0
	}
	return len(bytes)
}

func (sizer *JsonSizer) calculateMetricsSize(metrics pmetric.Metrics) int {
	bytes, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(metrics)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Any("obj", metrics))
		return 0
	}
	return len(bytes)
}

func (sizer *JsonSizer) calculateLogsSize(logs plog.Logs) int {
	bytes, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Any("obj", logs))
		return 0
	}

	return len(bytes)
}

func (sizer *JsonSizer) calculateMapStringAnySize(raw map[string]any) int {
	bytes, err := json.Marshal(raw)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Any("obj", raw))
		return 0
	}
	return len(bytes)
}
