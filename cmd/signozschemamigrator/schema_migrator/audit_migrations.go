package schemamigrator

import "github.com/SigNoz/signoz-otel-collector/utils"

var AuditMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: SignozAuditDB,
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
					{Name: "body", Type: JSONColumnType{Columns: []Column{{Name: "message", Type: ColumnTypeString}}, MaxDynamicPaths: utils.ToPointer[uint](0)}, Codec: "ZSTD(1)"},
					{
						Name: "scope",
						Type: JSONColumnType{
							MaxDynamicPaths: utils.ToPointer[uint](128),
							Columns: []Column{
								{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}},
								{Name: "version", Type: ColumnTypeString},
								{Name: "attributes", Type: JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)}},
							},
						},
						Codec: "ZSTD(1)",
					},
					{
						Name:  "attributes",
						Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)},
						Codec: "ZSTD(1)",
					},
					{
						Name: "resource",
						Type: JSONColumnType{
							MaxDynamicPaths: utils.ToPointer[uint](128),
							Columns: []Column{
								{Name: "attributes", Type: JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)}},
							},
						},
						Codec: "ZSTD(1)",
					},
					{Name: "event_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
					{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
				},
				Indexes: []Index{
					{Name: "id_minmax", Expression: "id", Type: "minmax", Granularity: 1},
					{Name: "severity_number_idx", Expression: "severity_number", Type: "set(25)", Granularity: 4},
					{Name: "severity_text_idx", Expression: "severity_text", Type: "set(25)", Granularity: 4},
					{Name: "body_string_ngram_idx", Expression: JSONFullTextIndexExpr("body"), Type: "ngrambf_v1(4, 15000, 3, 0)", Granularity: 1},
					{Name: "body_string_token_idx", Expression: JSONFullTextIndexExpr("body"), Type: "tokenbf_v1(10000, 2, 0)", Granularity: 1},
					{Name: "body_paths_ngram_idx", Expression: JSONPathsIndexExpr("body"), Type: "ngrambf_v1(4, 15000, 3, 0)", Granularity: 1},
					{Name: "body_paths_token_idx", Expression: JSONPathsIndexExpr("body"), Type: "tokenbf_v1(10000, 2, 0)", Granularity: 1}},
				Engine: MergeTree{
					PartitionBy: "(toDate(timestamp / 1000000000), _retention_days, _retention_days_cold)",
					OrderBy:     "(ts_bucket_start, resource_fingerprint, severity_text, timestamp, id)",
					TTL:         "toDateTime(timestamp / 1000000000) + toIntervalDay(_retention_days)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
						{Name: "object_serialization_version", Value: "'v3'"},
						{Name: "object_shared_data_serialization_version", Value: "'advanced'"},
					},
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
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
					{Name: "body", Type: JSONColumnType{Columns: []Column{{Name: "message", Type: ColumnTypeString}}, MaxDynamicPaths: utils.ToPointer[uint](0)}, Codec: "ZSTD(1)"},
					{
						Name: "scope",
						Type: JSONColumnType{
							MaxDynamicPaths: utils.ToPointer[uint](128),
							Columns: []Column{
								{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}},
								{Name: "version", Type: ColumnTypeString},
								{Name: "attributes", Type: JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)}},
							},
						},
						Codec: "ZSTD(1)",
					},
					{
						Name:  "attributes",
						Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)},
						Codec: "ZSTD(1)",
					},
					{
						Name: "resource",
						Type: JSONColumnType{
							MaxDynamicPaths: utils.ToPointer[uint](128),
							Columns: []Column{
								{Name: "attributes", Type: JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](128)}},
							},
						},
						Codec: "ZSTD(1)",
					},
					{Name: "event_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
					{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
				},
				Engine: Distributed{
					Database:    SignozAuditDB,
					Table:       "logs",
					ShardingKey: "cityHash64(id)",
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "field_keys",
				Columns: []Column{
					{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "field_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
					{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "(toDate(timestamp / 1000000000), _retention_days, _retention_days_cold)",
						OrderBy:     "(name, field_context, field_data_type)",
						TTL:         "toDateTime(timestamp / 1000000000) + toIntervalDay(_retention_days)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "distributed_field_keys",
				Columns: []Column{
					{Name: "timestamp", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "field_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
					{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
				},
				Engine: Distributed{
					Database:    SignozAuditDB,
					Table:       "field_keys",
					ShardingKey: "cityHash64(name, field_context, field_data_type)",
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "fields",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "field_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "value", Type: DynamicColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						// Dynamic is not allowed in a sorting key; without the value every key would collapse to one row.
						OrderBy: "(name, field_context, field_data_type, toString(value))",
						TTL:     "toDateTime(unix_milli / 1000) + toIntervalDay(365)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "distributed_fields",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "field_data_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "value", Type: DynamicColumnType{}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    SignozAuditDB,
					Table:       "fields",
					ShardingKey: "cityHash64(name, field_context, field_data_type)",
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "logs_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
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
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: SignozAuditDB,
				Table:    "distributed_logs_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "_retention_days", Type: ColumnTypeUInt16, Default: "365"},
					{Name: "_retention_days_cold", Type: ColumnTypeUInt16, Default: "0"},
				},
				Engine: Distributed{
					Database:    SignozAuditDB,
					Table:       "logs_resource",
					ShardingKey: "cityHash64(labels, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: SignozAuditDB, Table: "distributed_logs_resource"},
			DropTableOperation{Database: SignozAuditDB, Table: "logs_resource"},
			DropTableOperation{Database: SignozAuditDB, Table: "distributed_fields"},
			DropTableOperation{Database: SignozAuditDB, Table: "fields"},
			DropTableOperation{Database: SignozAuditDB, Table: "distributed_field_keys"},
			DropTableOperation{Database: SignozAuditDB, Table: "field_keys"},
			DropTableOperation{Database: SignozAuditDB, Table: "distributed_logs"},
			DropTableOperation{Database: SignozAuditDB, Table: "logs"},
		},
	},
}
