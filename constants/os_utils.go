package constants

import "os"

// Version is the current version of the collector.
// This is set at build time.
var Version = "dev"
var Desc = "SigNoz OpenTelemetry Collector"

// AllowLbExporterConfig enables lb exporter capability in the collector instance
var SupportLbExporterConfig = GetOrDefaultEnv("SUPPORT_LB_EXPORTER_CONFIG", "1")
var BodyColumnCompatibilityEnabled = GetOrDefaultEnv("BODY_COLUMN_COMPATIBILITY", "false") == "true"

func GetOrDefaultEnv(key string, fallback string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return fallback
	}
	return v
}
