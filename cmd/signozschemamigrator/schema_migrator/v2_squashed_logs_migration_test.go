package schemamigrator

import (
	"testing"
)

func TestCustomRetentionLogsMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, CustomRetentionLogsMigrations)
}

func TestCustomRetentionLogsMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		CustomRetentionLogsMigrations,
		[]SchemaMigrationRecord{},
	)
}
