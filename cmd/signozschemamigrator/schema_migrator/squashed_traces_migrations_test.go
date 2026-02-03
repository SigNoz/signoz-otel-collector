package schemamigrator

import (
	"testing"
)

func TestSquashedTracesMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, SquashedTracesMigrations)
}

func TestSquashedTracesMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		SquashedTracesMigrations,
		[]SchemaMigrationRecord{},
	)
}
