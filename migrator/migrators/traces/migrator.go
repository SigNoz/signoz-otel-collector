package traces

import (
	"context"

	basemigrator "github.com/SigNoz/signoz-otel-collector/migrator/migrators/baseMigrator"
)

const (
	name            = "traces"
	database        = "signoz_traces"
	migrationFolder = "migrator/migrators/traces/migrations"
)

type TracesMigrator struct {
	*basemigrator.BaseMigrator
}

func (m *TracesMigrator) Migrate(ctx context.Context) error {
	return m.BaseMigrator.Migrate(ctx, database, migrationFolder)
}

func (m *TracesMigrator) Name() string {
	return name
}