package traces

import (
	"context"

	basemigrator "github.com/SigNoz/signoz-otel-collector/migrationManager/migrators/baseMigrator"
)

const (
	name            = "traces"
	database        = "signoz_traces"
	migrationFolder = "migrationManager/migrators/traces/migrations"
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
