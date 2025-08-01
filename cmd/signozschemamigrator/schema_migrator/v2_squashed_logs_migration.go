package schemamigrator

// Added _retention_days materialized column which is added _retention_days in partition key
var (
	CustomRetentionLogsMigrations = []SchemaMigrationRecord{
		{
			MigrationID: 1,
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
			MigrationID: 2,
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
			MigrationID: 3,
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
			MigrationID: 4,
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
			MigrationID: 5,
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
			MigrationID: 6,
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
			MigrationID: 7,
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
						{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "15"},
						{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
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
						PartitionBy: "(toDate(timestamp / 1000000000), _retention_days, _retention_days_cold)",
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
			MigrationID: 8,
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
						{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "15"},
						{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
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
			MigrationID: 9,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_logs",
					Table:    "logs_v2_resource",
					Columns: []Column{
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
						{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "15"},
						{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
					},
					Indexes: []Index{
						{Name: "idx_labels", Expression: "lower(labels)", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
						{Name: "idx_labels_v1", Expression: "labels", Type: "ngrambf_v1(4, 1024, 3, 0)", Granularity: 1},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							PartitionBy: "(toDate(seen_at_ts_bucket_start), _retention_days, _retention_days_cold)",
							OrderBy:     "(labels, fingerprint, seen_at_ts_bucket_start)",
							TTL:         "toDateTime(seen_at_ts_bucket_start) + toIntervalDay(_retention_days) + toIntervalSecond(1800)",
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
						{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "15"},
						{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
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
