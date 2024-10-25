package schemamigrator

var TracesMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Columns: []Column{
					{Name: "ts_bucket_start", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "resource_fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
					{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "traceState", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
					{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},

					{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// custom columns
					{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
					{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// attribute cols
					{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					{Name: "references", Type: ColumnTypeString, Codec: "ZSTD(1)"},
				},
				Indexes: []Index{
					{Name: "idx_id", Expression: "id", Type: "minmax", Granularity: 1},
					{Name: "idx_traceID", Expression: "traceID", Type: "tokenbf_v1(5000, 3,0)", Granularity: 1},
					{Name: "idx_spanID", Expression: "spanID", Type: "tokenbf_v1(5000, 3,0)", Granularity: 1},
					{Name: "idx_duration", Expression: "durationNano", Type: "minmax", Granularity: 1},
					{Name: "idx_name", Expression: "name", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "idx_kind", Expression: "kind", Type: "minmax", Granularity: 4},
					{Name: "idx_httpRoute", Expression: "httpRoute", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpUrl", Expression: "httpUrl", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpHost", Expression: "httpHost", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpMethod", Expression: "httpMethod", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_timestamp", Expression: "timestamp", Type: "minmax", Granularity: 1},
					{Name: "idx_rpcMethod", Expression: "rpcMethod", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_responseStatusCode", Expression: "responseStatusCode", Type: "set(0)", Granularity: 1},
					{Name: "idx_statusCodeString", Expression: "statusCodeString", Type: "set(3)", Granularity: 4},
					{Name: "idx_spanKind", Expression: "spanKind", Type: "set(5)", Granularity: 4},
					{Name: "attributes_string_idx_key", Expression: "mapKeys(attributes_string)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "attributes_string_idx_val", Expression: "mapValues(attributes_string)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "attributes_number_idx_key", Expression: "mapKeys(attributes_number)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "attributes_number_idx_val", Expression: "mapValues(attributes_number)", Type: "bloom_filter", Granularity: 1},
					{Name: "attributes_bool_idx_key", Expression: "mapKeys(attributes_bool)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "resources_string_idx_key", Expression: "mapKeys(resources_string)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "resources_string_idx_val", Expression: "mapValues(resources_string)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
				},
				Engine: MergeTree{
					PartitionBy: "toDate(timestamp)",
					OrderBy:     "(ts_bucket_start, resource_fingerprint, hasError, name, timestamp, id)",
					TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Columns: []Column{
					{Name: "ts_bucket_start", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
					{Name: "resource_fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
					{Name: "id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "traceState", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
					{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},

					{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// custom columns
					{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
					{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// attribute cols
					{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					{Name: "references", Type: ColumnTypeString, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_traces",
					Table:       "signoz_index_v3",
					ShardingKey: "cityHash64(traceID)",
				},
			},
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "traces_v3_resource",
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
				Database: "signoz_traces",
				Table:    "distributed_traces_v3_resource",
				Columns: []Column{
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "seen_at_ts_bucket_start", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_traces",
					Table:       "traces_v3_resource",
					ShardingKey: "cityHash64(labels, fingerprint)",
				},
			},
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "trace_summary",
				Columns: []Column{
					{Name: "traceID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "first_reported",
						Type: SimpleAggregateFunction{
							FunctionName: "min",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "last_reported",
						Type: SimpleAggregateFunction{
							FunctionName: "max",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "num_spans",
						Type: SimpleAggregateFunction{
							FunctionName: "sum",
							Arguments:    []ColumnType{ColumnTypeUInt64},
						},
						Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(first_reported)",
						OrderBy:     "(traceID)",
						TTL:         "toDateTime(first_reported) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_trace_summary",
				Columns: []Column{
					{Name: "traceID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "first_reported",
						Type: SimpleAggregateFunction{
							FunctionName: "min",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "last_reported",
						Type: SimpleAggregateFunction{
							FunctionName: "max",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "num_spans",
						Type: SimpleAggregateFunction{
							FunctionName: "sum",
							Arguments:    []ColumnType{ColumnTypeUInt64},
						},
						Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_traces",
					Table:       "trace_summary",
					ShardingKey: "cityHash64(traceID)",
				},
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_traces",
				ViewName:  "trace_summary_mv",
				DestTable: "trace_summary",
				Query: `SELECT
							traceID,
							minSimpleState(timestamp) AS first_reported,
							maxSimpleState(timestamp) AS last_reported,
							sumSimpleState(toUInt64(1)) AS num_spans
						FROM signoz_traces.signoz_index_v3
						GROUP BY traceID;`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "root_operations",
				Query: `SELECT DISTINCT
							name,
							serviceName
						FROM signoz_traces.signoz_index_v3
						WHERE parentSpanID = ''`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "sub_root_operations",
				Query: `SELECT DISTINCT
							name,
							serviceName
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.serviceName != B.serviceName) AND (A.parentSpanID = B.spanID)`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "logs_v2_resource",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2_resource",
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "root_operations",
				Query: `SELECT DISTINCT
							name,
							serviceName
						FROM signoz_traces.signoz_index_v2
						WHERE parentSpanID = ''`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "sub_root_operations",
				Query: `SELECT DISTINCT
							name,
							serviceName
						FROM signoz_traces.signoz_index_v2 AS A, signoz_traces.signoz_index_v2 AS B
						WHERE (A.serviceName != B.serviceName) AND (A.parentSpanID = B.spanID)`,
			},
		},
	},
}
