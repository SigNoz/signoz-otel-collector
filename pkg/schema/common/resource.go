package common

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func ServiceName(input pcommon.Resource) string {
	service, found := input.Attributes().Get(semconv.AttributeServiceName)
	if !found {
		return "<nil-service-name>"
	}

	return service.Str()
}
