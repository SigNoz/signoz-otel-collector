package schemamigrator

var (
	SquashedLogsMigrations = []SchemaMigrationRecord{
		{
			MigrationID: 1,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "logs",
					Columns: []Column{
						{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "observed_timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "trace_flags", Type: ColumnTypeUInt32},
						{Name: "severity_text", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "severity_number", Type: ColumnTypeUInt8},
						{Name: "body", Type: ColumnTypeString, Codec: "ZSTD(2)"},
						{Name: "resources_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "resources_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_int64_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_int64_value", Type: ArrayColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
						{Name: "attributes_float64_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_float64_value", Type: ArrayColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "attributes_bool_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_bool_value", Type: ArrayColumnType{ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "scope_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "scope_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "id_minmax", Expression: "id", Type: "minmax", Granularity: 1},
						{Name: "severity_number_idx", Expression: "severity_number", Type: "set(25)", Granularity: 4},
						{Name: "severity_text_idx", Expression: "severity_text", Type: "set(25)", Granularity: 4},
						{Name: "trace_flags_idx", Expression: "trace_flags", Type: "bloom_filter", Granularity: 4},
						{Name: "body_idx", Expression: "lower(body)", Type: "ngrambf_v1(4, 60000, 5, 0)", Granularity: 1},
						{Name: "scope_name_idx", Expression: "scope_name", Type: "tokenbf_v1(10240, 3, 0)", Granularity: 4},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp / 1000000000)",
						OrderBy:     "(timestamp, id)",
						TTL:         "toDateTime(timestamp / 1000000000) + toIntervalSecond(1296000)",
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
					Table:    "logs",
				},
			},
		},
		{
			MigrationID: 2,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs",
					Columns: []Column{
						{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "observed_timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "trace_flags", Type: ColumnTypeUInt32},
						{Name: "severity_text", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "severity_number", Type: ColumnTypeUInt8},
						{Name: "body", Type: ColumnTypeString, Codec: "ZSTD(2)"},
						{Name: "resources_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "resources_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_int64_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_int64_value", Type: ArrayColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
						{Name: "attributes_float64_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_float64_value", Type: ArrayColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "attributes_bool_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "attributes_bool_value", Type: ArrayColumnType{ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "scope_string_key", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "scope_string_value", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_logs",
						Table:       "logs",
						ShardingKey: "cityHash64(id)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs",
				},
			},
		},
		{
			MigrationID: 3,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Engine: ReplacingMergeTree{
						MergeTree{
							OrderBy: "(name, datatype)",
							Settings: TableSettings{
								{Name: "index_granularity", Value: "8192"},
							},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "logs_attribute_keys",
				},
			},
		},
		{
			MigrationID: 4,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Engine: Distributed{
						Database:    "signoz_logs",
						Table:       "logs_attribute_keys",
						ShardingKey: "cityHash64(datatype)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs_attribute_keys",
				},
			},
		},
		{
			MigrationID: 5,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_logs",
					ViewName:  "attribute_keys_bool_final_mv",
					DestTable: "logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Query: `SELECT DISTINCT
    arrayJoin(attributes_bool_key) AS name,
    'Bool' AS datatype
FROM signoz_logs.logs
ORDER BY name ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_bool_final_mv",
				},
			},
		},
		{
			MigrationID: 6,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_logs",
					ViewName:  "attribute_keys_float64_final_mv",
					DestTable: "logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Query: `SELECT DISTINCT
    arrayJoin(attributes_float64_key) AS name,
    'Float64' AS datatype
FROM signoz_logs.logs
ORDER BY name ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_float64_final_mv",
				},
			},
		},
		{
			MigrationID: 7,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_logs",
					ViewName:  "attribute_keys_int64_final_mv",
					DestTable: "logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Query: `SELECT DISTINCT
    arrayJoin(attributes_int64_key) AS name,
    'Int64' AS datatype
FROM signoz_logs.logs
ORDER BY name ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_int64_final_mv",
				},
			},
		},
		{
			MigrationID: 8,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_logs",
					ViewName:  "attribute_keys_string_final_mv",
					DestTable: "logs_attribute_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Query: `SELECT DISTINCT
    arrayJoin(attributes_string_key) AS name,
    'String' AS datatype
FROM signoz_logs.logs
ORDER BY name ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_string_final_mv",
				},
			},
		},
		{
			MigrationID: 9,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "logs_resource_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Engine: ReplacingMergeTree{
						MergeTree{
							OrderBy: "(name, datatype)",
							Settings: TableSettings{
								{Name: "index_granularity", Value: "8192"},
							},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "logs_resource_keys",
				},
			},
		},
		{
			MigrationID: 10,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs_resource_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Engine: Distributed{
						Database:    "signoz_logs",
						Table:       "logs_resource_keys",
						ShardingKey: "cityHash64(datatype)",
					},
				},
			},
		},
		{
			MigrationID: 11,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_logs",
					ViewName:  "resource_keys_string_final_mv",
					DestTable: "logs_resource_keys",
					Columns: []Column{
						{Name: "name", Type: ColumnTypeString},
						{Name: "datatype", Type: ColumnTypeString},
					},
					Query: `SELECT DISTINCT
    arrayJoin(resources_string_key) AS name,
    'String' AS datatype
FROM signoz_logs.logs
ORDER BY name ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "resource_keys_string_final_mv",
				},
			},
		},
		{
			MigrationID: 12,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "tag_attributes",
					Columns: []Column{
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2", "'scope' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "tagDataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'int64' = 3", "'float64' = 4"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "stringTagValue", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "int64TagValue", Type: NullableColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
						{Name: "float64TagValue", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							OrderBy: "(tagKey, tagType, tagDataType, stringTagValue, int64TagValue, float64TagValue)",
							TTL:     "toDateTime(timestamp) + toIntervalSecond(172800)",
							Settings: TableSettings{
								{Name: "index_granularity", Value: "8192"},
								{Name: "ttl_only_drop_parts", Value: "1"},
								{Name: "allow_nullable_key", Value: "1"},
							},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "tag_attributes",
				},
			},
		},
		{
			MigrationID: 13,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_tag_attributes",
					Columns: []Column{
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2", "'scope' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "tagDataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'int64' = 3", "'float64' = 4"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "stringTagValue", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "int64TagValue", Type: NullableColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
						{Name: "float64TagValue", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_logs",
						Table:       "tag_attributes",
						ShardingKey: "rand()",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_tag_attributes",
				},
			},
		},
		{
			MigrationID: 14,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "usage",
					Columns: []Column{
						{Name: "tenant", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "collector_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exporter_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "data", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: MergeTree{
						OrderBy: "(tenant, collector_id, exporter_id, timestamp)",
						TTL:     "timestamp + toIntervalDay(3)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "usage",
				},
			},
		},
		{
			MigrationID: 15,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_usage",
					Columns: []Column{
						{Name: "tenant", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "collector_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exporter_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "data", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_logs",
						Table:       "usage",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_usage",
				},
			},
		},
		{
			MigrationID: 16,
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
						PartitionBy: "toDate(timestamp / 1000000000)",
						OrderBy:     "(ts_bucket_start, resource_fingerprint, severity_text, timestamp, id)",
						TTL:         "toDateTime(timestamp / 1000000000) + toIntervalSecond(1296000)",
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
			MigrationID: 17,
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
			MigrationID: 18,
			UpItems: []Operation{
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "resource_keys_string_final_mv",
				},
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_float64_final_mv",
				},
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_int64_final_mv",
				},
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_string_final_mv",
				},
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "attribute_keys_bool_final_mv",
				},
			},
		},
		{
			MigrationID: 19,
			UpItems: []Operation{
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
			MigrationID: 20,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "logs_v2_resource",
					Columns: []Column{
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
						{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "idx_labels", Expression: "lower(labels)", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
						{Name: "idx_labels_v1", Expression: "labels", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							PartitionBy: "toDate(seen_at_ts_bucket_start / 1000)",
							OrderBy:     "(labels, fingerprint, seen_at_ts_bucket_start)",
							TTL:         "toDateTime(seen_at_ts_bucket_start) + INTERVAL 1296000 SECOND + INTERVAL 1800 SECOND DELETE",
							Settings: TableSettings{
								{Name: "ttl_only_drop_parts", Value: "1"},
								{Name: "index_granularity", Value: "8192"},
							},
						},
					},
				},
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs_v2_resource",
					Columns: []Column{
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
						{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
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
					Table:    "logs_v2_resource",
				},
				DropTableOperation{
					Database: "signoz_logs",
					Table:    "distributed_logs_v2_resource",
				},
			},
		},
	}
)
