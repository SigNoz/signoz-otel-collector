package schemamigrator

import "testing"

func TestAuditMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, AuditMigrations)
}

func TestAuditMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		AuditMigrations,
		[]SchemaMigrationRecord{},
	)
}
