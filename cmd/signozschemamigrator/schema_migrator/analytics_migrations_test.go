package schemamigrator

import "testing"

func TestAnalyticsMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, AnalyticsMigrations)
}

func TestAnalyticsMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			AnalyticsMigrations[0],
		},
		[]SchemaMigrationRecord{},
	)
}
