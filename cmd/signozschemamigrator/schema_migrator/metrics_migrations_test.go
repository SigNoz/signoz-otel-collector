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
			MetricsMigrations[8],  // 1008 (sync)
			MetricsMigrations[9],  // 1009 (sync)
			MetricsMigrations[10], // 1010 (sync)
			MetricsMigrations[11], // 1011 (sync)
			MetricsMigrations[12], // 1012 (sync)
		},
		[]SchemaMigrationRecord{
			MetricsMigrations[5], // 1005 (async)
			MetricsMigrations[6], // 1006 (async)
		},
	)
}
