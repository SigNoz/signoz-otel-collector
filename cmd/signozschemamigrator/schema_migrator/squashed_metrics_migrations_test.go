package schemamigrator

import (
	"testing"
)

func TestSquashedMetricsMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, SquashedMetricsMigrations)
}

func TestSquashedMetricsMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		SquashedMetricsMigrations,
		[]SchemaMigrationRecord{},
	)
}
