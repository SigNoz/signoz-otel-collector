package schemamigrator

import (
	"testing"
)

func TestSquashedLogsMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, SquashedLogsMigrations)
}

func TestSquashedLogsMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		//remove 17th from SquashedLogsMigrations,
		[]SchemaMigrationRecord{
			SquashedLogsMigrations[0],
			SquashedLogsMigrations[1],
			SquashedLogsMigrations[2],
			SquashedLogsMigrations[3],
			SquashedLogsMigrations[4],
			SquashedLogsMigrations[5],
			SquashedLogsMigrations[6],
			SquashedLogsMigrations[7],
			SquashedLogsMigrations[8],
			SquashedLogsMigrations[9],
			SquashedLogsMigrations[10],
			SquashedLogsMigrations[11],
			SquashedLogsMigrations[12],
			SquashedLogsMigrations[13],
			SquashedLogsMigrations[14],
			SquashedLogsMigrations[15],
			SquashedLogsMigrations[16],
			SquashedLogsMigrations[18],
			SquashedLogsMigrations[19],
		},
		[]SchemaMigrationRecord{
			SquashedLogsMigrations[17], // 18 (async)
		},
	)
}
