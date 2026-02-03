package schemamigrator

import (
	"testing"
)

func TestMeterMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, MeterMigrations)
}

func TestMeterMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			MeterMigrations[0],
			MeterMigrations[1],
			MeterMigrations[2],
			MeterMigrations[3],
			MeterMigrations[4],
		},
		[]SchemaMigrationRecord{},
	)
}
