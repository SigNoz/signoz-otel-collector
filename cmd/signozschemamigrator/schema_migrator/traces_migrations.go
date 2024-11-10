package schemamigrator

// move them to TracesMigrations once it's ready to deploy
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
					{Name: "id", Type: FixedStringColumnType{Length: 27}, Codec: "ZSTD"},
					{Name: "trace_id", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_state", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parent_span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "span_kind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "duration_nano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					{Name: "status_code", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
					{Name: "status_message", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "status_code_string", Type: ColumnTypeString, Codec: "ZSTD(1)"},

					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},

					{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},
					{Name: "links", Type: ColumnTypeString, Codec: "ZSTD(1)"},

					// custom composite columns
					{Name: "response_status_code", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "external_http_url", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_url", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "external_http_method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_host", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "db_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "db_operation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "has_error", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
					{Name: "is_remote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// simple resource attribute
					{Name: "resource_string_service$$name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// simple attributes
					{Name: "attribute_string_http$$route", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$operation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_db$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_peer$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// ----- ALIAS for backward compatibility ------------
					// ALIAS columns to maintain compatibility with signoz_index_v2
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Alias: "trace_id"},
					{Name: "spanID", Type: ColumnTypeString, Alias: "span_id"},
					{Name: "parentSpanID", Type: ColumnTypeString, Alias: "parent_span_id"},
					{Name: "spanKind", Type: ColumnTypeString, Alias: "span_kind"},
					{Name: "durationNano", Type: ColumnTypeUInt64, Alias: "duration_nano"},
					{Name: "statusCode", Type: ColumnTypeInt16, Alias: "status_code"},
					{Name: "statusMessage", Type: ColumnTypeString, Alias: "status_message"},
					{Name: "statusCodeString", Type: ColumnTypeString, Alias: "status_code_string"},

					{Name: "references", Type: ColumnTypeString, Alias: "links"},

					// ALIAS for composite attrs
					{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "response_status_code"},
					{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "external_http_url"},
					{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_url"},
					{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "external_http_method"},
					{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_method"},
					{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_host"},
					{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "db_name"},
					{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "db_operation"},
					{Name: "hasError", Type: ColumnTypeBool, Alias: "has_error"},
					{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "is_remote"},

					{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "resource_string_service$$name"},

					// ALIAS for simple attrs
					{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_http$$route"},
					{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_messaging$$system"},
					{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_messaging$$operation"},
					{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_db$$system"},
					{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$system"},
					{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$service"},
					{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$method"},
					{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_peer$$service"},
				},
				Indexes: []Index{
					{Name: "idx_id", Expression: "id", Type: "minmax", Granularity: 1},
					{Name: "idx_trace_id", Expression: "trace_id", Type: "tokenbf_v1(10000, 5,0)", Granularity: 1},
					{Name: "idx_spanID", Expression: "span_id", Type: "tokenbf_v1(5000, 5,0)", Granularity: 1},
					{Name: "idx_duration", Expression: "duration_nano", Type: "minmax", Granularity: 1},
					{Name: "idx_name", Expression: "name", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "idx_kind", Expression: "kind", Type: "minmax", Granularity: 4},
					{Name: "idx_httpRoute", Expression: "attribute_string_http$$route", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpUrl", Expression: "http_url", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpHost", Expression: "http_host", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_httpMethod", Expression: "http_method", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_timestamp", Expression: "timestamp", Type: "minmax", Granularity: 1},
					{Name: "idx_rpcMethod", Expression: "attribute_string_rpc$$method", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_responseStatusCode", Expression: "response_status_code", Type: "set(0)", Granularity: 1},
					{Name: "idx_statusCodeString", Expression: "status_code_string", Type: "set(3)", Granularity: 4},
					{Name: "idx_spanKind", Expression: "span_kind", Type: "set(5)", Granularity: 4},
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
					OrderBy:     "(ts_bucket_start, resource_fingerprint, has_error, name, timestamp, id)",
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
					{Name: "id", Type: FixedStringColumnType{Length: 27}, Codec: "ZSTD"},
					{Name: "trace_id", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_state", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parent_span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "span_kind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "duration_nano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					{Name: "status_code", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
					{Name: "status_message", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "status_code_string", Type: ColumnTypeString, Codec: "ZSTD(1)"},

					{Name: "attributes_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attributes_number", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
					{Name: "attributes_bool", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeBool}, Codec: "ZSTD(1)"},
					{Name: "resources_string", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},

					{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},
					{Name: "links", Type: ColumnTypeString, Codec: "ZSTD(1)"},

					// custom composite columns
					{Name: "response_status_code", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "external_http_url", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_url", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "external_http_method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "http_host", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "db_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "db_operation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "has_error", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
					{Name: "is_remote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// simple resource attribute
					{Name: "resource_string_service$$name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// simple attributes
					{Name: "attribute_string_http$$route", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$operation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_db$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$method", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attribute_string_peer$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},

					// ----- ALIAS for backward compatibility ------------
					// ALIAS columns to maintain compatibility with signoz_index_v2
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Alias: "trace_id"},
					{Name: "spanID", Type: ColumnTypeString, Alias: "span_id"},
					{Name: "parentSpanID", Type: ColumnTypeString, Alias: "parent_span_id"},
					{Name: "spanKind", Type: ColumnTypeString, Alias: "span_kind"},
					{Name: "durationNano", Type: ColumnTypeUInt64, Alias: "duration_nano"},
					{Name: "statusCode", Type: ColumnTypeInt16, Alias: "status_code"},
					{Name: "statusMessage", Type: ColumnTypeString, Alias: "status_message"},
					{Name: "statusCodeString", Type: ColumnTypeString, Alias: "status_code_string"},

					{Name: "references", Type: ColumnTypeString, Alias: "links"},

					// ALIAS for composite attrs
					{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "response_status_code"},
					{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "external_http_url"},
					{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_url"},
					{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "external_http_method"},
					{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_method"},
					{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "http_host"},
					{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "db_name"},
					{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "db_operation"},
					{Name: "hasError", Type: ColumnTypeBool, Alias: "has_error"},
					{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "is_remote"},

					{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "resource_string_service$$name"},

					// ALIAS for simple attrs
					{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_http$$route"},
					{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_messaging$$system"},
					{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_messaging$$operation"},
					{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_db$$system"},
					{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$system"},
					{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$service"},
					{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_rpc$$method"},
					{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Alias: "attribute_string_peer$$service"},
				},
				Engine: Distributed{
					Database:    "signoz_traces",
					Table:       "signoz_index_v3",
					ShardingKey: "cityHash64(trace_id)",
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
					{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "start",
						Type: SimpleAggregateFunction{
							FunctionName: "min",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "end",
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
						PartitionBy: "toDate(start)",
						OrderBy:     "(trace_id)",
						TTL:         "toDateTime(start) + toIntervalSecond(1296000)",
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
					{Name: "trace_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "start",
						Type: SimpleAggregateFunction{
							FunctionName: "min",
							Arguments:    []ColumnType{DateTime64ColumnType{Precision: 9}},
						},
						Codec: "ZSTD(1)"},
					{Name: "end",
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
					ShardingKey: "cityHash64(trace_id)",
				},
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_traces",
				ViewName:  "trace_summary_mv",
				DestTable: "trace_summary",
				Query: `SELECT
							trace_id,
							min(timestamp) AS start,
							max(timestamp) AS end,
							toUInt64(count()) AS num_spans
						FROM signoz_traces.signoz_index_v3
						GROUP BY trace_id;`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "root_operations",
				Query: `SELECT DISTINCT
							name,
							resource_string_service$$name as serviceName
						FROM signoz_traces.signoz_index_v3
						WHERE parent_span_id = ''`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "sub_root_operations",
				Query: `SELECT DISTINCT
							name,
							resource_string_service$$name as serviceName
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.parent_span_id = B.span_id)`,
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
				Database: "signoz_traces",
				Table:    "traces_v3_resource",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_traces_v3_resource",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "trace_summary",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_trace_summary",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "trace_summary_mv",
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
						WHERE (A.serviceName != B.serviceName) AND (A.parentSpanId = B.spanID)`,
			},
		},
	},
}
