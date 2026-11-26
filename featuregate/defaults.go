package featuregate

import (
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"
)

// DisabledByDefault lists the feature gates that upstream turned on by default
// after v0.144.0. We keep them off so the collector behaves as it did on v0.144.0.
// Any of them can still be turned on with --feature-gates=<id>.
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

// ApplyDefaults turns off the gates in DisabledByDefault. Call it before the
// --feature-gates flag is parsed so the flag can override these defaults.
func ApplyDefaults(reg *featuregate.Registry) error {
	var errs error
	for _, id := range DisabledByDefault {
		errs = multierr.Append(errs, reg.Set(id, false))
	}
	return errs
}
