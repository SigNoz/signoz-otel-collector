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

func TestCustomRetentionLogsMigrationsGuardZeroDayTTL(t *testing.T) {
	want := map[string]string{
		"logs_v2":          "toDateTime(timestamp / 1000000000) + toIntervalDay(if(_retention_days = 0, 30, _retention_days))",
		"logs_v2_resource": "toDateTime(seen_at_ts_bucket_start) + toIntervalDay(if(_retention_days = 0, 30, _retention_days)) + toIntervalSecond(1800)",
	}

	for _, migration := range CustomRetentionLogsMigrations {
		for _, operation := range migration.UpItems {
			create, ok := operation.(CreateTableOperation)
			if !ok {
				continue
			}
			if ttl, ok := want[create.Table]; ok {
				var got string
				switch engine := create.Engine.(type) {
				case MergeTree:
					got = engine.TTL
				case ReplacingMergeTree:
					got = engine.TTL
				default:
					t.Fatalf("%s uses unexpected engine %T", create.Table, create.Engine)
				}
				if got != ttl {
					t.Fatalf("%s TTL = %q, want %q", create.Table, got, ttl)
				}
				delete(want, create.Table)
			}
		}
	}
	for table := range want {
		t.Errorf("migration does not create %s", table)
	}
}
