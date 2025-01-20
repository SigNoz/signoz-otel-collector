package schemamigrator

// database := "CREATE DATABASE IF NOT EXISTS signoz_analytics ON CLUSTER %s"

var AnalyticsMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_analytics",
				Table:    "rule_state_history_v0",
				Columns: []Column{
					{Name: "_retention_days", Type: ColumnTypeUInt32, Default: "180"},
					{Name: "rule_id", Type: ColumnTypeString},
					{Name: "rule_name", Type: ColumnTypeString},
					{Name: "overall_state", Type: ColumnTypeString},
					{Name: "overall_state_changed", Type: ColumnTypeBool},
					{Name: "state", Type: ColumnTypeString},
					{Name: "state_changed", Type: ColumnTypeBool},
					{Name: "unix_milli", Type: ColumnTypeInt64},
					{Name: "fingerprint", Type: ColumnTypeUInt64},
					{Name: "value", Type: ColumnTypeFloat64},
					{Name: "labels", Type: ColumnTypeString},
				},
				Engine: MergeTree{
					PartitionBy: "toDate(unix_milli / 1000)",
					OrderBy:     "(rule_id, unix_milli)",
					TTL:         "toDateTime(unix_milli / 1000) + toIntervalDay(_retention_days)",
					Settings: TableSettings{
						{Name: "ttl_only_drop_parts", Value: "1"},
						{Name: "index_granularity", Value: "8192"},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_analytics",
				Table:    "distributed_rule_state_history_v0",
				Columns: []Column{
					{Name: "rule_id", Type: ColumnTypeString},
					{Name: "rule_name", Type: ColumnTypeString},
					{Name: "overall_state", Type: ColumnTypeString},
					{Name: "overall_state_changed", Type: ColumnTypeBool},
					{Name: "state", Type: ColumnTypeString},
					{Name: "state_changed", Type: ColumnTypeBool},
					{Name: "unix_milli", Type: ColumnTypeInt64},
					{Name: "fingerprint", Type: ColumnTypeUInt64},
					{Name: "value", Type: ColumnTypeFloat64},
					{Name: "labels", Type: ColumnTypeString},
				},
				Engine: Distributed{
					Database:    "signoz_analytics",
					Table:       "rule_state_history_v0",
					ShardingKey: "cityHash64(rule_id, rule_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_analytics",
				Table:    "distributed_rule_state_history_v0",
			},
			DropTableOperation{
				Database: "signoz_analytics",
				Table:    "rule_state_history_v0",
			},
		},
	},
}
