package signozspanmetricsconnector

import (
	"time"

	"go.opentelemetry.io/collector/featuregate"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// connector config and factory constants
const (
	delta                                  = "AGGREGATION_TEMPORALITY_DELTA"
	cumulative                             = "AGGREGATION_TEMPORALITY_CUMULATIVE"
	maxNumberOfServicesToTrack             = 256
	maxNumberOfOperationsToTrackPerService = 2048
	defaultDimensionsCacheSize             = 1000
	defaultSkipSpansOlderThan              = 24 * time.Hour
	defaultTimeBucketInterval              = time.Minute
	dropSanitizationGateID                 = "processor.signozspanmetrics.PermissiveLabelSanitization"
)

// connector implementation constants
const (
	serviceNameKey = conventions.AttributeServiceName
	// TODO(nikhilmantri0902): rename to span.name
	operationKey                   = "operation"                          // OpenTelemetry non-standard constant.
	spanKindKey                    = "span.kind"                          // OpenTelemetry non-standard constant.
	statusCodeKey                  = "status.code"                        // OpenTelemetry non-standard constant.
	collectorInstanceKey           = "collector.instance.id"              // OpenTelemetry non-standard constant.
	instrumentationScopeNameKey    = "span.instrumentation.scope.name"    // OpenTelemetry non-standard constant.
	instrumentationScopeVersionKey = "span.instrumentation.scope.version" // OpenTelemetry non-standard constant.
	metricKeySeparator             = string(byte(0))
	signozID                       = "signoz.collector.id"

	tagHTTPStatusCode       = conventions.AttributeHTTPStatusCode
	tagHTTPStatusCodeStable = "http.response.status_code"

	overflowServiceName = "overflow_service"
	overflowOperation   = "overflow_operation"

	resourcePrefix = "resource_"
)

// connector implementation variables
var (
	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
	}
	dropSanitizationFeatureGate *featuregate.Gate
)

func init() {
	dropSanitizationFeatureGate = featuregate.GlobalRegistry().MustRegister(
		dropSanitizationGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Controls whether to change labels starting with '_' to 'key_'"),
	)
}
