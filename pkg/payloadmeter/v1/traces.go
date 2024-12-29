package v1

import (
	"errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/payloadmeter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traceMeter struct {
	Sizer payloadmeter.Sizer
}

func NewTraceMeter(logger *zap.Logger, serialization payloadmeter.Serialization) (payloadmeter.Traces, error) {
	if serialization == payloadmeter.Json {
		return &traceMeter{
			Sizer: payloadmeter.NewJSONSizer(logger),
		}, nil
	} else if serialization == payloadmeter.Proto {
		return &traceMeter{
			Sizer: payloadmeter.NewProtoSizer(logger),
		}, nil
	} else {
		return nil, errors.New("serialization not supported")
	}
}

func (meter *traceMeter) Size(t ptrace.Traces, d payloadmeter.Decoding) int {
	total := 0
	if d == payloadmeter.Otlp {
		return meter.Sizer.GetSize(t)
	} else if d == payloadmeter.Signoz {
		//TODO
	}
	return total
}
func (*traceMeter) Count(t ptrace.Traces, _ payloadmeter.Decoding) int {
	return t.SpanCount()
}
