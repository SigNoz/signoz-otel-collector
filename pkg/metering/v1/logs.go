package v1

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/metering"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logs struct {
	Logger *zap.Logger
	Sizer  metering.Sizer
}

func NewLogs(logger *zap.Logger) metering.Logs {
	return &logs{
		Logger: logger,
		Sizer:  metering.NewJSONSizer(logger),
	}
}

func (meter *logs) Size(ld plog.Logs) int {

	meter.Logger.Debug("Calculating logs size")

	total := 0
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		resourceAttributesSize := meter.Sizer.SizeOfMapStringAny(resourceLog.Resource().Attributes().AsRaw())

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLog.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				total += resourceAttributesSize +
					meter.Sizer.SizeOfMapStringAny(logRecord.Attributes().AsRaw()) +
					len([]byte(logRecord.Body().AsString()))
			}

		}
	}
	meter.Logger.Debug("Logs size", zap.Int("size", total))

	return total
}
func (*logs) Count(ld plog.Logs) int {
	return ld.LogRecordCount()
}
