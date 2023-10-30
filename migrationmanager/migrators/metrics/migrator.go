package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter"
	"github.com/SigNoz/signoz-otel-collector/migrationmanager/migrators/basemigrator"
)

const (
	name            = "metrics"
	database        = "signoz_metrics"
	migrationFolder = "migrationmanager/migrators/metrics/migrations"
)

type MetricsMigrator struct {
	*basemigrator.BaseMigrator
}

func (m *MetricsMigrator) Migrate(ctx context.Context) error {
	err := m.BaseMigrator.Migrate(ctx, database, migrationFolder)
	if err != nil {
		return err
	}

	// TODO(srikanthccv): Remove this once we have a better way to handle data and last write
	removeTTL := fmt.Sprintf(`ALTER TABLE %s.%s ON CLUSTER %s REMOVE TTL;`, database, clickhousemetricsexporter.TIME_SERIES_TABLE, m.Cfg.ClusterName)
	if err = m.DB.Exec(context.Background(), removeTTL); err != nil {
		if !strings.Contains(err.Error(), "Table doesn't have any table TTL expression, cannot remove.") {
			return fmt.Errorf("failed to remove TTL from table %s.%s, err: %s", database, clickhousemetricsexporter.TIME_SERIES_TABLE, err)
		}
	}
	return nil
}

func (m *MetricsMigrator) Name() string {
	return name
}
