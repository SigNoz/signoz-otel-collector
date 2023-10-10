package logs

import (
	"context"
	basemigrator "github.com/SigNoz/signoz-otel-collector/migrator/migrators/baseMigrator"
)

const (
	name            = "logs"
	database        = "signoz_logs"
	migrationFolder = "migrator/migrators/logs/migrations"
)

type LogsMigrator struct {
	*basemigrator.BaseMigrator
}

func (m *LogsMigrator) Migrate(ctx context.Context) error {
	return m.BaseMigrator.Migrate(ctx, database, migrationFolder)
}

func (m *LogsMigrator) Name() string {
	return name
}
