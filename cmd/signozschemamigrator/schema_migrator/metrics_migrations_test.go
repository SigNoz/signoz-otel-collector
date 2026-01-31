package schemamigrator

import (
	"testing"
)

func TestMetricsMigrations(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigratorMigrationRecords(t, manager, MetricsMigrations)
}

func TestMetricsMigrationsExactNature(t *testing.T) {
	manager := newTestMigrationManager(t)
	checkSchemaMigrationRecordExactNature(
		t,
		manager,
		[]SchemaMigrationRecord{
			MetricsMigrations[0],
			MetricsMigrations[1],
			MetricsMigrations[2],
			MetricsMigrations[3],
			MetricsMigrations[4],
			MetricsMigrations[7],
		},
		[]SchemaMigrationRecord{
			MetricsMigrations[5], // 1005 (async)
			MetricsMigrations[6], // 1006 (async)
		},
	)
}
