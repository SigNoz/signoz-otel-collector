package schemamigrator

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
)

// Run against a disposable ClickHouse server (25.12.5, as used by SigNoz):
// SIGNOZ_TEST_CLICKHOUSE_DSN=clickhouse://localhost:9000 go test ./cmd/signozschemamigrator/schema_migrator -run TestMetricFieldKeysProjectionIntegration -v
// Set SIGNOZ_TEST_CLICKHOUSE_CLUSTER to a single-node cluster pointing at that
// server to also exercise the Distributed table used by getMetricsKeys.
func TestMetricFieldKeysProjectionIntegration(t *testing.T) {
	dsn := os.Getenv("SIGNOZ_TEST_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("set SIGNOZ_TEST_CLICKHOUSE_DSN to run the ClickHouse integration test")
	}
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database := fmt.Sprintf("metric_keys_test_%d", time.Now().UnixNano())
	require.NoError(t, conn.Exec(ctx, "CREATE DATABASE "+database))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		require.NoError(t, conn.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+database+" SYNC"))
	})

	// Use the real pre-migration table definition, including its aggregating
	// engine, value-level sorting key, daily partitions and retention policy.
	for _, op := range MetricsMigrations[0].UpItems {
		if create, ok := op.(CreateTableOperation); ok && create.Table == "metadata" {
			create.Database = database
			require.NoError(t, conn.Exec(ctx, create.ToSQL()))
		}
	}
	table := database + ".metadata"
	if cluster := os.Getenv("SIGNOZ_TEST_CLICKHOUSE_CLUSTER"); cluster != "" {
		engine := Distributed{Cluster: cluster, Database: database, Table: "metadata", ShardingKey: "rand()"}
		require.NoError(t, conn.Exec(ctx, "CREATE TABLE "+database+".distributed_metadata AS "+table+" ENGINE = "+engine.ToSQL()))
		table = database + ".distributed_metadata"
	}

	// Keep all current fixture rows in the same daily partition even if the
	// test happens to cross midnight.
	now := uint64(time.Now().UnixMilli())
	require.NoError(t, conn.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.metadata
		SELECT 'Cumulative', concat(if(number %% 2 = 0, 'k8s.', 'other.'), toString(number %% 100)),
			'', '', 'Gauge', false,
			if(number %% 50 = 0, '__internal', concat('field_', toString(intDiv(number, 100) %% 20))),
			arrayElement(['resource', 'scope', 'point'], intDiv(number, 2000) %% 3 + 1),
			'string', toString(number),
			toUInt64(?), toUInt64(?)
		FROM numbers(200000)`, database), now, now))

	// Keep the nested grouping and priority expression identical to SigNoz's
	// getMetricsKeys: the optimization must work without changing the API query.
	query := func(predicate string) string {
		return fmt.Sprintf(`SELECT name, field_context, field_data_type, max(priority) AS priority
			FROM (SELECT attr_name AS name, attr_type AS field_context, attr_datatype AS field_data_type,
				CASE WHEN attr_type = 'resource' THEN 1 WHEN attr_type = 'scope' THEN 2
				WHEN attr_type = 'point' THEN 3 ELSE 4 END AS priority
				FROM %s WHERE (%s) AND attr_name NOT LIKE '\_\_%%'
				GROUP BY name, field_context, field_data_type) AS sub_query
			GROUP BY name, field_context, field_data_type ORDER BY priority LIMIT 1001`, table, predicate)
	}
	readKeys := func(query, id string) []string {
		t.Helper()
		queryCtx := clickhouse.Context(ctx, clickhouse.WithQueryID(id))
		rows, err := conn.Query(queryCtx, query)
		require.NoError(t, err)
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var name, fieldContext, dataType string
			var priority uint8
			require.NoError(t, rows.Scan(&name, &fieldContext, &dataType, &priority))
			keys = append(keys, fmt.Sprintf("%s;%s;%s;%d", name, fieldContext, dataType, priority))
		}
		require.NoError(t, rows.Err())
		sort.Strings(keys)
		return keys
	}
	predicates := []string{
		"LOWER(attr_name) LIKE LOWER('%%') AND metric_name LIKE 'k8s.%'",
		"(attr_name = 'field_1' OR attr_name = 'field_2') AND metric_name LIKE 'k8s.%'",
		"metric_name = 'k8s.2'",
		"LOWER(attr_name) LIKE LOWER('%FIELD_1%')",
		"attr_name = '__internal'",
		"metric_name = 'missing'",
	}
	baseline := make([][]string, len(predicates))
	for idx, predicate := range predicates {
		baseline[idx] = readKeys(query(predicate), fmt.Sprintf("%s_before_%d", database, idx))
	}
	require.NotEmpty(t, baseline[0])
	require.Empty(t, baseline[4], "internal fields must remain hidden")

	applyMigration := func(id uint64, down bool) {
		t.Helper()
		for _, migration := range MetricsMigrations {
			if migration.MigrationID != id {
				continue
			}
			operations := migration.UpItems
			if down {
				operations = migration.DownItems
			}
			for _, op := range operations {
				// Only replace the database identifier, retaining the actual DDL.
				ddl := strings.ReplaceAll(op.ToSQL(), "signoz_metrics.", database+".")
				if op.IsMutation() {
					ddl += " SETTINGS mutations_sync = 1"
				}
				require.NoError(t, conn.Exec(ctx, ddl))
			}
			return
		}
		t.Fatalf("migration %d not found", id)
	}
	applyMigration(1012, false)
	// Creating the projection must not hide historical, unmaterialized parts.
	require.Equal(t, baseline[0], readKeys(query(predicates[0]), database+"_pending"))
	applyMigration(1013, false)

	for idx, predicate := range predicates {
		require.Equal(t, baseline[idx], readKeys(query(predicate), fmt.Sprintf("%s_after_%d", database, idx)))
	}
	require.NoError(t, conn.Exec(ctx, "SYSTEM FLUSH LOGS"))
	var beforeRows, afterRows uint64
	var projections []string
	require.NoError(t, conn.QueryRow(ctx, `SELECT read_rows FROM system.query_log
		WHERE type = 'QueryFinish' AND is_initial_query = 1 AND query_id = ?`, database+"_before_0").Scan(&beforeRows))
	require.NoError(t, conn.QueryRow(ctx, `SELECT read_rows, projections FROM system.query_log
		WHERE type = 'QueryFinish' AND is_initial_query = 1 AND query_id = ?`, database+"_after_0").Scan(&afterRows, &projections))
	require.Contains(t, projections, database+".metadata.metric_field_keys")
	require.Less(t, afterRows, beforeRows/10, "key discovery should not scale with the number of attribute values")
	t.Logf("broad metric key discovery: read_rows before=%d after=%d", beforeRows, afterRows)

	// New keys must appear immediately and survive AggregatingMergeTree merges.
	require.NoError(t, conn.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.metadata
		(temporality, metric_name, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli)
		VALUES ('Cumulative', 'k8s.new', 'new_key', 'resource', 'string', 'new_value', ?, ?)`, database),
		now, now))
	updated := readKeys(query(predicates[0]), database+"_insert")
	require.Contains(t, updated, "new_key;resource;string;1")
	// Exercise a merge that combines duplicate value identities as well as parts.
	require.NoError(t, conn.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.metadata
		SELECT * FROM %s.metadata WHERE attr_name = 'new_key'`, database, database)))
	require.NoError(t, conn.Exec(ctx, "OPTIMIZE TABLE "+database+".metadata FINAL"))
	require.Equal(t, updated, readKeys(query(predicates[0])+" SETTINGS force_optimize_projection = 1", database+"_merged"))
	var values uint64
	require.NoError(t, conn.QueryRow(ctx, "SELECT count() FROM "+database+".metadata WHERE attr_name = 'new_key'").Scan(&values))
	require.Equal(t, uint64(1), values, "the base table must still aggregate duplicate metadata values")

	// Projections share the parent part's lifetime. Dropping an old partition
	// must remove its keys, just as the table's ttl_only_drop_parts policy does.
	yesterday := now - uint64((24 * time.Hour).Milliseconds())
	require.NoError(t, conn.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.metadata
		(temporality, metric_name, attr_name, attr_type, attr_datatype, attr_string_value, first_reported_unix_milli, last_reported_unix_milli)
		VALUES ('Cumulative', 'k8s.expired', 'expired_key', 'resource', 'string', 'old_value', ?, ?)`, database), yesterday, yesterday))
	require.Contains(t, readKeys(query(predicates[0]), database+"_old_part"), "expired_key;resource;string;1")
	var partition string
	require.NoError(t, conn.QueryRow(ctx, "SELECT _partition_id FROM "+database+".metadata WHERE attr_name = 'expired_key' LIMIT 1").Scan(&partition))
	require.NoError(t, conn.Exec(ctx, "ALTER TABLE "+database+".metadata DROP PARTITION ID ?", partition))
	require.Equal(t, updated, readKeys(query(predicates[0])+" SETTINGS force_optimize_projection = 1", database+"_drop_part"))

	// A retried backfill and rollback must preserve query results.
	applyMigration(1013, false)
	applyMigration(1012, true)
	require.Equal(t, updated, readKeys(query(predicates[0]), database+"_rollback"))
}
