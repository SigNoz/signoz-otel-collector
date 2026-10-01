package metadataexporter

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/internal/fieldvalues"
)

// newFieldValuesWriter creates the writer of the field values store for one
// signal. The shared day cache uses the Redis connection of the key cache.
// Body pairs follow the JSON config, so they are written only where the JSON
// writer runs today.
func newFieldValuesWriter(cfg Config, set exporter.Settings, signal pipeline.Signal, conn driver.Conn) (*fieldvalues.Writer, error) {
	settings := fieldvalues.Settings{
		Signal:    signal,
		Conn:      conn,
		Logger:    set.Logger,
		Telemetry: set.TelemetrySettings,
	}
	if cfg.FieldValues.Cache.Provider == fieldvalues.CacheProviderRedis {
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.Cache.Redis.Addr,
			Username: cfg.Cache.Redis.Username,
			Password: cfg.Cache.Redis.Password,
			DB:       cfg.Cache.Redis.DB,
		})
		settings.Shared = fieldvalues.NewRedisCache(client, cfg.TenantID, signal.String(), cfg.FieldValues.Source)
	}
	if signal == pipeline.SignalLogs && cfg.JSON.Enabled {
		settings.BodyJSON = &fieldvalues.BodyJSONLimits{
			MaxDepthTraverse:        *cfg.JSON.MaxDepthTraverse,
			MaxArrayElementsAllowed: *cfg.JSON.MaxArrayElementsAllowed,
			MaxKeysAtLevel:          *cfg.JSON.MaxKeysAtLevel,
		}
	}
	return fieldvalues.New(cfg.FieldValues, settings)
}

// fieldValuesLogsWriter runs the field values writer as one of the logs
// metadata writers.
type fieldValuesLogsWriter struct {
	w *fieldvalues.Writer
}

func (f fieldValuesLogsWriter) Process(ctx context.Context, ld plog.Logs) error {
	return f.w.WriteLogs(ctx, ld)
}
