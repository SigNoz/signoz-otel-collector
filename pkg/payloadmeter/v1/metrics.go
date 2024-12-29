package v1

import (
	"errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/payloadmeter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricMeter struct {
	Sizer payloadmeter.Sizer
}

func NewMetricMeter(logger *zap.Logger, serialization payloadmeter.Serialization) (payloadmeter.Metrics, error) {
	if serialization == payloadmeter.Json {
		return &metricMeter{
			Sizer: payloadmeter.NewJSONSizer(logger),
		}, nil
	} else if serialization == payloadmeter.Proto {
		return &metricMeter{
			Sizer: payloadmeter.NewProtoSizer(logger),
		}, nil
	} else {
		return nil, errors.New("serialization not supported")
	}
}

func (meter *metricMeter) Size(m pmetric.Metrics, d payloadmeter.Decoding) int {
	total := 0
	if d == payloadmeter.Otlp {
		return meter.Sizer.GetSize(m)
	} else if d == payloadmeter.Signoz {
		//TODO implement deep calculation of bytes based on how we do the billing for metrics
	}
	return total
}
func (*metricMeter) Count(m pmetric.Metrics, _ payloadmeter.Decoding) int {
	return m.MetricCount()
}
