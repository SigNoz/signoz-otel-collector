package featuregate

import (
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"
)

// DisabledByDefault can be overridden with --feature-gates.
var DisabledByDefault = []string{
	"connector.spanmetrics.includeCollectorInstanceID",
	"datadog.EnableScopeConvention",
	"exporter.datadogexporter.AddUnits",
	"exporter.datadogexporter.DisableAllMetricRemapping",
	"exporter.datadogexporter.metricremappingdisabled",
	"exporter.prometheus.DisableResourceToTelemetryConversion",
	"extension.encoding.googlecloudlogentryencoding.DontEmitV0RPCConventions",
	"filelog.windows.caseInsensitive",
	"pkg.stanza.addDefaultScopeName",
	"pkg.translator.azurelogs.DontEmitV0LogConventions",
	"pkg.translator.azurelogs.EmitV1LogConventions",
	"pkg.translator.jaeger.DontEmitV0HttpConventions",
	"pkg.translator.jaeger.EmitV1HttpConventions",
	"pkg.translator.zipkin.DontEmitV0CloudResourceConventions",
	"pkg.translator.zipkin.DontEmitV0HttpConventions",
	"pkg.translator.zipkin.DontEmitV0ScopeConventions",
	"pkg.translator.zipkin.EmitV1CloudResourceConventions",
	"pkg.translator.zipkin.EmitV1HttpConventions",
	"pkg.translator.zipkin.EmitV1ScopeConventions",
	"processor.k8sattributes.DontEmitV0K8sConventions",
	"processor.k8sattributes.EmitV1K8sConventions",
	"processor.k8sattributes.telemetry.disableOldFormatMetrics",
	"processor.k8sattributes.telemetry.enableNewFormatMetrics",
	"receiver.awsxray.DontEmitV0HttpConventions",
	"receiver.awsxray.EmitV1HttpConventions",
	"receiver.datadogreceiver.Enable128BitTraceID",
	"receiver.githubreceiver.UseCheckRunID",
	"receiver.hostmetricsreceiver.UseLinuxMemAvailable",
	"receiver.kubeletstats.cpuUsageScrapeBased",
	"receiver.vcenter.resourcePoolMemoryUsageAttribute",
	"service.partialReloadReceivers",
}

// ApplyDefaults must run before --feature-gates is parsed.
func ApplyDefaults(reg *featuregate.Registry) error {
	var errs error
	for _, id := range DisabledByDefault {
		errs = multierr.Append(errs, reg.Set(id, false))
	}
	return errs
}
