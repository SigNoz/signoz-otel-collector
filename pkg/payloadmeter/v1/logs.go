package v1

import (
	"errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/payloadmeter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsMeter struct {
	Sizer payloadmeter.Sizer
}

func NewLogsMeter(logger *zap.Logger, serialization payloadmeter.Serialization) (payloadmeter.Logs, error) {
	if serialization == payloadmeter.Json {
		return &logsMeter{
			Sizer: payloadmeter.NewJSONSizer(logger),
		}, nil
	} else if serialization == payloadmeter.Proto {
		return &logsMeter{
			Sizer: payloadmeter.NewProtoSizer(logger),
		}, nil
	} else {
		return nil, errors.New("serialization not supported")
	}
}

func (meter *logsMeter) Size(ld plog.Logs, d payloadmeter.Decoding) int {
	total := 0
	if d == payloadmeter.Otlp {
		return meter.Sizer.GetSize(ld)
	} else if d == payloadmeter.Signoz {
		//Implement deep calculation of bytes based on how we do the billing for logs
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			resourceLog := ld.ResourceLogs().At(i)
			resourceAttributesSize := meter.Sizer.GetSize(resourceLog.Resource().Attributes().AsRaw())

			for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
				scopeLogs := resourceLog.ScopeLogs().At(j)

				for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
					logRecord := scopeLogs.LogRecords().At(k)
					total += resourceAttributesSize +
						meter.Sizer.GetSize(logRecord.Attributes().AsRaw()) +
						len([]byte(logRecord.Body().AsString()))
				}

			}
		}
	}
	return total
}
func (*logsMeter) Count(ld plog.Logs, _ payloadmeter.Decoding) int {
	return ld.LogRecordCount()
}
