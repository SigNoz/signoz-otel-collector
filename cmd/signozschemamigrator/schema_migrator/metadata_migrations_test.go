package schemamigrator

import (
	"testing"
)

func TestMetadataMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, MetadataMigrations)
}

func TestMetadataMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			MetadataMigrations[0],
			MetadataMigrations[1],
		},
		[]SchemaMigrationRecord{},
	)

}
