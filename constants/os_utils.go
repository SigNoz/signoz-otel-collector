package constants

import "os"

// AllowLbExporterConfig enables lb exporter capability in the collector instance
var AllowLbExporterConfig = GetOrDefaultEnv("ALLOW_LB_EXPORTER_CONFIG", "1")

func GetOrDefaultEnv(key string, fallback string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return fallback
	}
	return v
}
