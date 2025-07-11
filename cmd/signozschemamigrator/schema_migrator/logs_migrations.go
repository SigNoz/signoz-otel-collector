package schemamigrator

var LogsMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_bool_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_float64_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_string_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "resource_keys_string_final_mv",
			},
		},
		DownItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_bool_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_bool)) AS name,
'Bool' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_float64_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_number)) AS name,
'Float64' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_string_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_string)) AS name,
'String' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "resource_keys_string_final_mv",
				DestTable: "logs_resource_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(resources_string)) AS name,
'String' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
		},
	},
	{
		MigrationID: 1001,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "tag_attributes_v2",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "tag_key", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "tag_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "tag_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "number_value", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
				},
				Indexes: []Index{
					{Name: "string_value_index", Expression: "string_value", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
					{Name: "number_value_index", Expression: "number_value", Type: "minmax", Granularity: 1},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(tag_key, tag_type, tag_data_type, string_value, number_value)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
							{Name: "allow_nullable_key", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_tag_attributes_v2",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "tag_key", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "tag_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "tag_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "number_value", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "tag_attributes_v2",
					ShardingKey: "cityHash64(rand())",
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1002,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				Column: Column{
					Name:    "timestamp",
					Type:    DateTimeColumnType{},
					Default: "toDateTime(now())",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				Column: Column{
					Name:    "timestamp",
					Type:    DateTimeColumnType{},
					Default: "toDateTime(now())",
				},
			},
			AlterTableModifyTTL{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				TTL:      "timestamp + INTERVAL 15 DAY",
				Settings: ModifyTTLSettings{
					MaterializeTTLAfterModify: false,
				},
			},
			AlterTableModifyTTL{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				TTL:      "timestamp + INTERVAL 15 DAY",
				Settings: ModifyTTLSettings{
					MaterializeTTLAfterModify: false,
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1003,
		UpItems: []Operation{
			AlterTableMaterializeColumn{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				Column: Column{
					Name: "timestamp",
				},
			},
			AlterTableMaterializeColumn{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				Column: Column{
					Name: "timestamp",
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1004,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Columns: []Column{
					{Name: "ts_bucket_start", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "resource_fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "observed_timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_flags", Type: ColumnTypeUInt32},
					{Name: "severity_text", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "severity_number", Type: ColumnTypeUInt8},
					{Name: "body", Type: ColumnTypeString, Codec: "ZSTD(2)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Materialized: "15"},
				},
				Indexes: []Index{
					{Name: "id_minmax", Expression: "id", Type: "minmax", Granularity: 1},
					{Name: "severity_number_idx", Expression: "severity_number", Type: "set(25)", Granularity: 4},
					{Name: "severity_text_idx", Expression: "severity_text", Type: "set(25)", Granularity: 4},
					{Name: "trace_flags_idx", Expression: "trace_flags", Type: "bloom_filter", Granularity: 4},
					{Name: "body_idx", Expression: "lower(body)", Type: "ngrambf_v1(4, 60000, 5, 0)", Granularity: 1},
					{Name: "scope_name_idx", Expression: "scope_name", Type: "tokenbf_v1(10240, 3, 0)", Granularity: 4},
					{Name: "attributes_string_idx_key", Expression: "mapKeys(attributes_string)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "attributes_string_idx_val", Expression: "mapValues(attributes_string)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "attributes_number_idx_key", Expression: "mapKeys(attributes_number)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "attributes_number_idx_val", Expression: "mapValues(attributes_number)", Type: "bloom_filter", Granularity: 1},
					{Name: "attributes_bool_idx_key", Expression: "mapKeys(attributes_bool)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
				},
				Engine: MergeTree{
					PartitionBy: "(toDate(timestamp / 1000000000), _retention_days)",
					OrderBy:     "(ts_bucket_start, resource_fingerprint, severity_text, timestamp, id)",
					TTL:         "toDateTime(timestamp / 1000000000) + toIntervalDay(_retention_days)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "logs_v2",
			},
		},
	},
	{
		MigrationID: 1005,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
				Columns: []Column{
					{Name: "ts_bucket_start", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "resource_fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "observed_timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_flags", Type: ColumnTypeUInt32},
					{Name: "severity_text", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "severity_number", Type: ColumnTypeUInt8},
					{Name: "body", Type: ColumnTypeString, Codec: "ZSTD(2)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Materialized: "15"},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "logs_v2",
					ShardingKey: "cityHash64(id)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
			},
		},
	},
	{
		MigrationID: 1006,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "logs_v2_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Materialized: "15"},
				},
				Indexes: []Index{
					{Name: "idx_labels", Expression: "lower(labels)", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
					{Name: "idx_labels_v1", Expression: "labels", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "(toDate(seen_at_ts_bucket_start / 1000), _retention_days)",
						OrderBy:     "(labels, fingerprint, seen_at_ts_bucket_start)",
						TTL:         "toDateTime(seen_at_ts_bucket_start) + toIntervalDay(_retention_days) + toIntervalSecond(1800)",
						Settings: TableSettings{
							{Name: "ttl_only_drop_parts", Value: "1"},
							{Name: "index_granularity", Value: "8192"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "logs_v2_resource",
			},
		},
	},
	{
		MigrationID: 1007,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Materialized: "15"},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "logs_v2_resource",
					ShardingKey: "cityHash64(labels, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2_resource",
			},
		},
	},
}
