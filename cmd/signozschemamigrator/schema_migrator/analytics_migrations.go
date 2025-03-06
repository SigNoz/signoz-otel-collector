package schemamigrator

var AnalyticsMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_analytics",
				Table:    "rule_state_history_v0",
				Columns: []Column{
					{Name: "_retention_days", Type: ColumnTypeUInt32, Default: "180"},
					{Name: "rule_id", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "rule_name", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "overall_state", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "overall_state_changed", Type: ColumnTypeBool},
					{Name: "state", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "state_changed", Type: ColumnTypeBool},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
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
					{Name: "rule_id", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "rule_name", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "overall_state", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "overall_state_changed", Type: ColumnTypeBool},
					{Name: "state", Type: LowCardinalityColumnType{ElementType: ColumnTypeString}},
					{Name: "state_changed", Type: ColumnTypeBool},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
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
