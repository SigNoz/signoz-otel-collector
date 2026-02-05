package schemamigrator

import "testing"

func TestLogsMigrationsV2(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, LogsMigrationsV2)
}

func TestLogsMigrationsV2ExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			LogsMigrationsV2[1],
			LogsMigrationsV2[2],
			LogsMigrationsV2[4],
			LogsMigrationsV2[5],
		},
		[]SchemaMigrationRecord{
			LogsMigrationsV2[0], // 1000 (async)
			LogsMigrationsV2[3], // 1003 (async)
		},
	)
}
