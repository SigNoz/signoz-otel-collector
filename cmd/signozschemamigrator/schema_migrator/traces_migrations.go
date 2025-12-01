package schemamigrator

import "github.com/SigNoz/signoz-otel-collector/utils"

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
					{Name: "trace_id", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_state", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parent_span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "kind_string", Type: ColumnTypeString, Codec: "ZSTD(1)"},
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
					{Name: "resource_string_service$$name", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "resources_string['service.name']", Codec: "ZSTD(1)"},

					// simple attributes
					{Name: "attribute_string_http$$route", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['http.route']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['messaging.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$operation", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['messaging.operation']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_db$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['db.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.service']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$method", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.method']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_peer$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['peer.service']", Codec: "ZSTD(1)"},

					// ----- ALIAS for backward compatibility ------------
					// ---------------------------------------------------

					// ALIAS columns to maintain compatibility with signoz_index_v2
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Alias: "trace_id"},
					{Name: "spanID", Type: ColumnTypeString, Alias: "span_id"},
					{Name: "parentSpanID", Type: ColumnTypeString, Alias: "parent_span_id"},
					{Name: "spanKind", Type: ColumnTypeString, Alias: "kind_string"},
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
					{Name: "idx_trace_id", Expression: "trace_id", Type: "tokenbf_v1(10000, 5,0)", Granularity: 1},
					{Name: "idx_span_id", Expression: "span_id", Type: "tokenbf_v1(5000, 5,0)", Granularity: 1},
					{Name: "idx_duration", Expression: "duration_nano", Type: "minmax", Granularity: 1},
					{Name: "idx_name", Expression: "name", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "idx_kind", Expression: "kind", Type: "minmax", Granularity: 4},
					{Name: "idx_http_route", Expression: "attribute_string_http$$route", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_http_url", Expression: "http_url", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_http_host", Expression: "http_host", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_http_method", Expression: "http_method", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_timestamp", Expression: "timestamp", Type: "minmax", Granularity: 1},
					{Name: "idx_rpc_method", Expression: "attribute_string_rpc$$method", Type: "bloom_filter", Granularity: 4},
					{Name: "idx_response_statusCode", Expression: "response_status_code", Type: "set(0)", Granularity: 1},
					{Name: "idx_status_code_string", Expression: "status_code_string", Type: "set(3)", Granularity: 4},
					{Name: "idx_kind_string", Expression: "kind_string", Type: "set(5)", Granularity: 4},
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
					OrderBy:     "(ts_bucket_start, resource_fingerprint, has_error, name, timestamp)",
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
					{Name: "trace_id", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
					{Name: "span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "trace_state", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "parent_span_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "T64, ZSTD(1)"},
					{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
					{Name: "kind_string", Type: ColumnTypeString, Codec: "ZSTD(1)"},
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
					{Name: "resource_string_service$$name", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "resources_string['service.name']", Codec: "ZSTD(1)"},

					// simple attributes
					{Name: "attribute_string_http$$route", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['http.route']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['messaging.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_messaging$$operation", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['messaging.operation']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_db$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['db.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$system", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.system']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.service']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_rpc$$method", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['rpc.method']", Codec: "ZSTD(1)"},
					{Name: "attribute_string_peer$$service", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "attributes_string['peer.service']", Codec: "ZSTD(1)"},

					// ----- ALIAS for backward compatibility ------------
					// ---------------------------------------------------

					// ALIAS columns to maintain compatibility with signoz_index_v2
					{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Alias: "trace_id"},
					{Name: "spanID", Type: ColumnTypeString, Alias: "span_id"},
					{Name: "parentSpanID", Type: ColumnTypeString, Alias: "parent_span_id"},
					{Name: "spanKind", Type: ColumnTypeString, Alias: "kind_string"},
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
						PartitionBy: "toDate(seen_at_ts_bucket_start)",
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
						PartitionBy: "toDate(end)",
						OrderBy:     "(trace_id)",
						TTL:         "toDateTime(end) + toIntervalSecond(1296000)",
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
						WHERE (A.serviceName != B.serviceName) AND (A.parentSpanID = B.spanID)`,
			},
		},
	},
	{
		MigrationID: 1001,
		UpItems: []Operation{
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "durationSortMV",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_durationSort",
			},
			// DropTableOperation{
			// 	Database: "signoz_traces",
			// 	Table:    "durationSort",
			// 	// this is added so that we can avoid the following error
			// 	//1. Size (453.51 GB) is greater than max_[table/partition]_size_to_drop (50.00 GB)
			// 	// https://stackoverflow.com/questions/78162269/cannot-drop-large-materialized-view-in-clickhouse
			// 	Settings: TableSettings{{Name: "max_table_size_to_drop", Value: "0"}},
			// },
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1002,
		UpItems: []Operation{
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "dependency_graph_minutes_db_calls_mv_v2",
				Query: `SELECT
							resource_string_service$$name AS src,
							attribute_string_db$$system AS dest,
							quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(duration_nano)) AS duration_quantiles_state,
							countIf(status_code = 2) AS error_count,
							count(*) AS total_count,
							toStartOfMinute(timestamp) AS timestamp,
							resources_string['deployment.environment'] AS deployment_environment,
							resources_string['k8s.cluster.name'] AS k8s_cluster_name,
							resources_string['k8s.namespace.name'] AS k8s_namespace_name
						FROM signoz_traces.signoz_index_v3
						WHERE (dest != '') AND (kind != 2)
						GROUP BY
							timestamp,
							src,
							dest,
							deployment_environment,
							k8s_cluster_name,
							k8s_namespace_name`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "dependency_graph_minutes_messaging_calls_mv_v2",
				Query: `SELECT
							resource_string_service$$name  AS src,
							attribute_string_messaging$$system AS dest,
							quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(duration_nano)) AS duration_quantiles_state,
							countIf(status_code = 2) AS error_count,
							count(*) AS total_count,
							toStartOfMinute(timestamp) AS timestamp,
							resources_string['deployment.environment'] AS deployment_environment,
							resources_string['k8s.cluster.name'] AS k8s_cluster_name,
							resources_string['k8s.namespace.name'] AS k8s_namespace_name
						FROM signoz_traces.signoz_index_v3
						WHERE (dest != '') AND (kind != 2)
						GROUP BY
							timestamp,
							src,
							dest,
							deployment_environment,
							k8s_cluster_name,
							k8s_namespace_name`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "dependency_graph_minutes_service_calls_mv_v2",
				Query: `SELECT
							A.resource_string_service$$name AS src,
							B.resource_string_service$$name AS dest,
							quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.duration_nano)) AS duration_quantiles_state,
							countIf(B.status_code = 2) AS error_count,
							count(*) AS total_count,
							toStartOfMinute(B.timestamp) AS timestamp,
							B.resources_string['deployment.environment'] AS deployment_environment,
							B.resources_string['k8s.cluster.name'] AS k8s_cluster_name,
							B.resources_string['k8s.namespace.name'] AS k8s_namespace_name
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.span_id = B.parent_span_id)
						GROUP BY
							timestamp,
							src,
							dest,
							deployment_environment,
							k8s_cluster_name,
							k8s_namespace_name`,
			},
		},
		DownItems: []Operation{
			// no point of down here as we don't want to go back
		},
	},
	{
		MigrationID: 1003,
		UpItems: []Operation{
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "dependency_graph_minutes_db_calls_mv",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "dependency_graph_minutes_messaging_calls_mv",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "dependency_graph_minutes_service_calls_mv",
			},
			DropTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_dependency_graph_minutes",
			},
			// remove dependency_graph_minutes later
		},
		DownItems: []Operation{
			// no point of down here as we don't use these
		},
	},
	{
		MigrationID: 1004,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_traces",
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
				Database: "signoz_traces",
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
					Database:    "signoz_traces",
					Table:       "tag_attributes_v2",
					ShardingKey: "cityHash64(rand())",
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1005,
		UpItems: []Operation{
			// Local Table
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "resource_string_service$$name_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(resources_string, 'service.name') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_http$$route_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'http.route') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_messaging$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'messaging.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_messaging$$operation_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'messaging.operation') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_db$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'db.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$service_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.service') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$method_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.method') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_peer$$service_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'peer.service') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},

			// Distributed Table
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "resource_string_service$$name_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(resources_string, 'service.name') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_http$$route_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'http.route') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_messaging$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'messaging.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_messaging$$operation_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'messaging.operation') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_db$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'db.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$system_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.system') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$service_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.service') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_rpc$$method_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'rpc.method') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:    "attribute_string_peer$$service_exists",
					Type:    ColumnTypeBool,
					Default: "if(mapContains(attributes_string, 'peer.service') != 0, true, false)",
					Codec:   "ZSTD(1)",
				},
			},
		},
		DownItems: []Operation{
			// Distributed table
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "resource_string_service$$name_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_http$$route_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_messaging$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_messaging$$operation_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_db$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$service_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$method_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "attribute_string_peer$$service_exists",
				},
			},

			// Local table
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "resource_string_service$$name_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_http$$route_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_messaging$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_messaging$$operation_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_db$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$system_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$service_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_rpc$$method_exists",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "attribute_string_peer$$service_exists",
				},
			},
		},
	},
	{
		MigrationID: 1006,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name:  "resource",
					Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer(uint(100))},
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name:  "resource",
					Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer(uint(100))},
					Codec: "ZSTD(1)",
				},
			},
		},
		DownItems: []Operation{
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "distributed_signoz_index_v3",
				Column: Column{
					Name: "resource",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "signoz_index_v3",
				Column: Column{
					Name: "resource",
				},
			},
		},
	},
	{
		MigrationID: 1007,
		UpItems: []Operation{
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "dependency_graph_minutes_service_calls_mv_v2",
				Query: `SELECT
							A.resource_string_service$$name AS src,
							B.resource_string_service$$name AS dest,
							quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.duration_nano)) AS duration_quantiles_state,
							countIf(B.status_code = 2) AS error_count,
							count(*) AS total_count,
							toStartOfMinute(B.timestamp) AS timestamp,
							B.resources_string['deployment.environment'] AS deployment_environment,
							B.resources_string['k8s.cluster.name'] AS k8s_cluster_name,
							B.resources_string['k8s.namespace.name'] AS k8s_namespace_name
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.span_id = B.parent_span_id)
							AND B.span_id != '' AND A.span_id != ''
						GROUP BY
							timestamp,
							src,
							dest,
							deployment_environment,
							k8s_cluster_name,
							k8s_namespace_name;`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "sub_root_operations",
				Query: `SELECT DISTINCT
							name,
							resource_string_service$$name AS serviceName
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.parent_span_id = B.span_id) AND B.span_id != '' AND A.span_id != ''`,
			},
		},
		DownItems: []Operation{
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "dependency_graph_minutes_service_calls_mv_v2",
				Query: `SELECT
							A.resource_string_service$$name AS src,
							B.resource_string_service$$name AS dest,
							quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.duration_nano)) AS duration_quantiles_state,
							countIf(B.status_code = 2) AS error_count,
							count(*) AS total_count,
							toStartOfMinute(B.timestamp) AS timestamp,
							B.resources_string['deployment.environment'] AS deployment_environment,
							B.resources_string['k8s.cluster.name'] AS k8s_cluster_name,
							B.resources_string['k8s.namespace.name'] AS k8s_namespace_name
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.span_id = B.parent_span_id)
						GROUP BY
							timestamp,
							src,
							dest,
							deployment_environment,
							k8s_cluster_name,
							k8s_namespace_name;`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_traces",
				ViewName: "sub_root_operations",
				Query: `SELECT DISTINCT
							name,
							resource_string_service$$name AS serviceName
						FROM signoz_traces.signoz_index_v3 AS A, signoz_traces.signoz_index_v3 AS B
						WHERE (A.resource_string_service$$name != B.resource_string_service$$name) AND (A.parent_span_id = B.span_id)`,
			},
		},
	},
	{
		MigrationID: 1008,
		UpItems: []Operation{
			// Add timestamp column to span_attributes_keys table
			AlterTableAddColumn{
				Database: "signoz_traces",
				Table:    "span_attributes_keys",
				Column: Column{
					Name:    "timestamp",
					Type:    DateTimeColumnType{},
					Default: "toDateTime(now())",
				},
			},
			// Set TTL on span_attributes_keys to match signoz_spans table (15 days)
			AlterTableModifyTTL{
				Database: "signoz_traces",
				Table:    "span_attributes_keys",
				TTL:      "timestamp + INTERVAL 15 DAY",
				Settings: ModifyTTLSettings{
					MaterializeTTLAfterModify: false,
				},
			},
		},
		DownItems: []Operation{
			AlterTableDropColumn{
				Database: "signoz_traces",
				Table:    "span_attributes_keys",
				Column: Column{
					Name: "timestamp",
				},
			},
			AlterTableDropTTL{
				Database: "signoz_traces",
				Table:    "span_attributes_keys",
			},
		},
	},
}
