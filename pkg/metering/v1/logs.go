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
		Sizer:  metering.NewJSONSizer(logger, metering.WithExcludePattern(metering.ExcludeSigNozWorkspaceResourceAttrs)),
	}
}

func (meter *logs) Size(ld plog.Logs) int {
	total := 0
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)
		total += meter.SizePerResource(resourceLog)
	}

	return total
}

func (meter *logs) SizePerResource(rld plog.ResourceLogs) int {
	total := 0
	resourceAttributesSize := meter.Sizer.SizeOfMapStringAny(rld.Resource().Attributes().AsRaw())

	for j := 0; j < rld.ScopeLogs().Len(); j++ {
		scopeLogs := rld.ScopeLogs().At(j)

		for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
			logRecord := scopeLogs.LogRecords().At(k)
			total += resourceAttributesSize +
				meter.Sizer.SizeOfMapStringAny(logRecord.Attributes().AsRaw()) +
				len([]byte(logRecord.Body().AsString()))
		}
	}

	return total
}

func (*logs) Count(ld plog.Logs) int {
	return ld.LogRecordCount()
}

func (meter *logs) CountPerResource(rld plog.ResourceLogs) int {
	logCount := 0

	ill := rld.ScopeLogs()
	for i := 0; i < ill.Len(); i++ {
		logs := ill.At(i)
		logCount += logs.LogRecords().Len()
	}

	return logCount
}
