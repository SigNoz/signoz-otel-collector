package schemamigrator

import (
	"testing"
)

func TestTracesMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, TracesMigrations)
}

func TestTracesMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			TracesMigrations[0],
			TracesMigrations[2],
			TracesMigrations[4],
			TracesMigrations[5],
			TracesMigrations[6],
			TracesMigrations[7],
			TracesMigrations[8],
		},
		[]SchemaMigrationRecord{
			TracesMigrations[1], // 1001 (async)
			TracesMigrations[3], // 1003 (async)
		},
	)
}
