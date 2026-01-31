package schemamigrator

import (
	"testing"

	mockhouse "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestMigrationManager(t *testing.T) *MigrationManager {
	t.Helper()

	conn, err := mockhouse.NewClickHouseNative(nil)
	require.NoError(t, err)

	manager, err := NewMigrationManager(
		WithClusterName("test"),
		WithReplicationEnabled(true),
		WithConn(conn),
		WithLogger(zap.NewNop()),
		WithDevelopment(false),
	)
	require.NoError(t, err)

	return manager
}

func checkSchemaMigratorMigrationRecords(t *testing.T, manager *MigrationManager, migrations []SchemaMigrationRecord) {
	t.Helper()
	assert.Greater(t, len(migrations), 0, "migrations array cannot be empty")

	seen := make(map[uint64]bool)
	for _, migration := range migrations {
		if ok := seen[migration.MigrationID]; ok {
			assert.Fail(t, "migration ID %d is duplicated", migration.MigrationID)
		}

		seen[migration.MigrationID] = true

		// check if the migration is sync or async
		checkSchemaMigrationRecordSyncOrAsync(t, manager, migration)
	}
}

func checkSchemaMigrationRecordSyncOrAsync(t *testing.T, manager *MigrationManager, migration SchemaMigrationRecord) {
	t.Helper()

	syncCount := 0
	asyncCount := 0
	neitherCount := 0

	for _, item := range migration.UpItems {
		syncOk := manager.IsSyncOperation(item)
		asyncOk := manager.IsAsyncOperation(item)

		if syncOk {
			syncCount++
		}

		if asyncOk {
			asyncCount++
		}

		if !syncOk && !asyncOk {
			neitherCount++
		}
	}

	if syncCount > 0 {
		assert.Equal(t, len(migration.UpItems), syncCount, "sync count should be equal to the number of operations")
	}

	if asyncCount > 0 {
		assert.Equal(t, len(migration.UpItems), asyncCount, "async count should be equal to the number of operations")
	}

	if neitherCount > 0 {
		assert.Equal(t, 0, neitherCount, "neither count should be zero")
	}
}

func checkSchemaMigrationRecordExactNature(t *testing.T, manager *MigrationManager, syncMigrations []SchemaMigrationRecord, asyncMigrations []SchemaMigrationRecord) {
	t.Helper()

	for _, migration := range syncMigrations {
		assert.True(t, manager.IsSync(migration))
	}

	for _, migration := range asyncMigrations {
		assert.True(t, manager.IsAsync(migration))
	}
}
