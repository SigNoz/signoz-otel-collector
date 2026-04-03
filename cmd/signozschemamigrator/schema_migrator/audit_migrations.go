package schemamigrator

var SignozAuditDB = "signoz_audit"

// AuditMigrations contains the schema migrations for the signoz_audit database.
// These tables follow the logs_v2 structure but are dedicated to audit logs with
// 365-day TTL and materialized columns for frequently queried audit attributes.
var AuditMigrations = []SchemaMigrationRecord{
	// Migration 1000: logs_attribute_keys + distributed_logs_attribute_keys
	{
		MigrationID: 1000,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "datatype", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy: "(name, datatype)",
						TTL:     "timestamp + toIntervalDay(365)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "datatype", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_audit",
					Table:       "logs_attribute_keys",
					ShardingKey: "cityHash64(name)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_attribute_keys",
			},
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "logs_attribute_keys",
			},
		},
	},
	// Migration 1001: logs_resource_keys + distributed_logs_resource_keys
	{
		MigrationID: 1001,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "logs_resource_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "datatype", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy: "(name, datatype)",
						TTL:     "timestamp + toIntervalDay(365)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_resource_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "datatype", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_audit",
					Table:       "logs_resource_keys",
					ShardingKey: "cityHash64(name)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_resource_keys",
			},
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "logs_resource_keys",
			},
		},
	},
	// Migration 1002: logs + distributed_logs (main audit event table)
	{
		MigrationID: 1002,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "logs",
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
					{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resource", Type: JSONColumnType{}, Codec: "ZSTD(1)"},
					{Name: "event_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					// Materialized DEFAULT columns for frequently queried audit attributes
					{Name: "`attribute_string_signoz$$audit$$principal$$id`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.id']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$id_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.id'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$email`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.email']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$email_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.email'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$type`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.type']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$type_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.type'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$action`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.action']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$action_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.action'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$outcome`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.outcome']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$outcome_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.outcome'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$name`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.resource.name']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$name_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.resource.name'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$id`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.resource.id']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$id_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.resource.id'), true, false)", Codec: "ZSTD(1)"},
				},
				Indexes: []Index{
					{Name: "id_minmax", Expression: "id", Type: "minmax", Granularity: 1},
					{Name: "severity_number_idx", Expression: "severity_number", Type: "set(25)", Granularity: 4},
					{Name: "severity_text_idx", Expression: "severity_text", Type: "set(25)", Granularity: 4},
					{Name: "body_idx", Expression: "lower(body)", Type: "tokenbf_v1(10240, 3, 0)", Granularity: 4},
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
					TTL:         "toDateTime(timestamp / 1000000000) + toIntervalDay(365)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs",
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
					{Name: "scope_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_version", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "scope_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resource", Type: JSONColumnType{}, Codec: "ZSTD(1)"},
					{Name: "event_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					// DEFAULT columns are included in distributed table (computed on local shard at insert time)
					{Name: "`attribute_string_signoz$$audit$$principal$$id`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.id']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$id_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.id'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$email`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.email']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$email_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.email'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$type`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.principal.type']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$principal$$type_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.principal.type'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$action`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.action']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$action_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.action'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$outcome`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.outcome']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$outcome_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.outcome'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$name`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.resource.name']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$name_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.resource.name'), true, false)", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$id`", Type: ColumnTypeString, Default: "attributes_string['signoz.audit.resource.id']", Codec: "ZSTD(1)"},
					{Name: "`attribute_string_signoz$$audit$$resource$$id_exists`", Type: ColumnTypeBool, Default: "if(mapContains(attributes_string, 'signoz.audit.resource.id'), true, false)", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_audit",
					Table:       "logs",
					ShardingKey: "cityHash64(id)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs",
			},
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "logs",
			},
		},
	},
	// Migration 1003: logs_resource + distributed_logs_resource
	{
		MigrationID: 1003,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "logs_resource",
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
						PartitionBy: "toDate(seen_at_ts_bucket_start)",
						OrderBy:     "(labels, fingerprint, seen_at_ts_bucket_start)",
						TTL:         "toDateTime(seen_at_ts_bucket_start) + toIntervalDay(365) + toIntervalSecond(1800)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_audit",
					Table:       "logs_resource",
					ShardingKey: "cityHash64(labels, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_logs_resource",
			},
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "logs_resource",
			},
		},
	},
	// Migration 1004: tag_attributes + distributed_tag_attributes
	{
		MigrationID: 1004,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "tag_attributes",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "tag_key", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "tag_type", Type: EnumerationColumnType{Values: []string{"'tag'", "'resource'", "'scope'"}, Size: 8}, Codec: "ZSTD(1)"},
					{Name: "tag_data_type", Type: EnumerationColumnType{Values: []string{"'string'", "'int64'", "'float64'", "'bool'"}, Size: 8}, Codec: "ZSTD(1)"},
					{Name: "string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "int64_value", Type: NullableColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
					{Name: "float64_value", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						OrderBy: "(tag_key, tag_type, tag_data_type, string_value, int64_value, float64_value)",
						TTL:     "toDateTime(unix_milli / 1000) + toIntervalDay(365)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
							{Name: "allow_nullable_key", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_tag_attributes",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "tag_key", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "tag_type", Type: EnumerationColumnType{Values: []string{"'tag'", "'resource'", "'scope'"}, Size: 8}, Codec: "ZSTD(1)"},
					{Name: "tag_data_type", Type: EnumerationColumnType{Values: []string{"'string'", "'int64'", "'float64'", "'bool'"}, Size: 8}, Codec: "ZSTD(1)"},
					{Name: "string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "int64_value", Type: NullableColumnType{ColumnTypeInt64}, Codec: "ZSTD(1)"},
					{Name: "float64_value", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_audit",
					Table:       "tag_attributes",
					ShardingKey: "cityHash64(tag_key)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "distributed_tag_attributes",
			},
			DropTableOperation{
				Database: "signoz_audit",
				Table:    "tag_attributes",
			},
		},
	},
}
