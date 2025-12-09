package constants

import "os"

// Version is the current version of the collector.
// This is set at build time.
var Version = "dev"
var Desc = "SigNoz OpenTelemetry Collector"

// AllowLbExporterConfig enables lb exporter capability in the collector instance
var SupportLbExporterConfig = GetOrDefaultEnv("SUPPORT_LB_EXPORTER_CONFIG", "1")

// EnableLogsMigrationsV2 enables logs migrations v2 (JSON migrations) if the environment variable is set to "1"
var EnableLogsMigrationsV2 = GetOrDefaultEnv("ENABLE_LOGS_MIGRATIONS_V2", "0") == "1"

func GetOrDefaultEnv(key string, fallback string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return fallback
	}
	return v
}
