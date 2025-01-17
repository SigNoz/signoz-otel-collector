package schemamigrator

var (
	SquashedTracesMigrations = []SchemaMigrationRecord{
		{
			MigrationID: 1,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_index_v2",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
						{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
						{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
						{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},
						{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "stringTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "numberTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "boolTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "resourceTagsMap", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "idx_service", Expression: "serviceName", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_name", Expression: "name", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_kind", Expression: "kind", Type: "minmax", Granularity: 4},
						{Name: "idx_duration", Expression: "durationNano", Type: "minmax", Granularity: 1},
						{Name: "idx_hasError", Expression: "hasError", Type: "set(2)", Granularity: 1},
						{Name: "idx_httpRoute", Expression: "httpRoute", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpUrl", Expression: "httpUrl", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpHost", Expression: "httpHost", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpMethod", Expression: "httpMethod", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_timestamp", Expression: "timestamp", Type: "minmax", Granularity: 1},
						{Name: "idx_rpcMethod", Expression: "rpcMethod", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_responseStatusCode", Expression: "responseStatusCode", Type: "set(0)", Granularity: 1},
						{Name: "idx_resourceTagsMapKeys", Expression: "mapKeys(resourceTagsMap)", Type: "bloom_filter(0.01)", Granularity: 64},
						{Name: "idx_resourceTagsMapValues", Expression: "mapValues(resourceTagsMap)", Type: "bloom_filter(0.01)", Granularity: 64},
						{Name: "idx_statusCodeString", Expression: "statusCodeString", Type: "set(3)", Granularity: 4},
						{Name: "idx_spanKind", Expression: "spanKind", Type: "set(5)", Granularity: 4},
						{Name: "idx_stringTagMapKeys", Expression: "mapKeys(stringTagMap)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
						{Name: "idx_stringTagMapValues", Expression: "mapValues(stringTagMap)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
						{Name: "idx_numberTagMapKeys", Expression: "mapKeys(numberTagMap)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
						{Name: "idx_numberTagMapValues", Expression: "mapValues(numberTagMap)", Type: "bloom_filter(0.01)", Granularity: 1},
						{Name: "idx_boolTagMapKeys", Expression: "mapKeys(boolTagMap)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
						{Name: "idx_resourceTagMapKeys", Expression: "mapKeys(resourceTagsMap)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
						{Name: "idx_resourceTagMapValues", Expression: "mapValues(resourceTagsMap)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					},
					Projections: []Projection{
						{Name: "timestampSort", Query: "SELECT * ORDER BY timestamp"},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp)",
						PrimaryKey:  "(serviceName, hasError, toStartOfHour(timestamp), name)",
						OrderBy:     "(serviceName, hasError, toStartOfHour(timestamp), name, timestamp)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_index_v2",
				},
			},
		},
		{
			MigrationID: 2,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_index_v2",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
						{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
						{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
						{Name: "externalHttpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "externalHttpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dbOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "peerService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "events", Type: ArrayColumnType{ColumnTypeString}, Codec: "ZSTD(2)"},
						{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "msgSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "msgOperation", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "stringTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "numberTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "boolTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "resourceTagsMap", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "signoz_index_v2",
						ShardingKey: "cityHash64(traceID)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_index_v2",
				},
			},
		},
		{
			MigrationID: 3,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "durationSort",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
						{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
						{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
						{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "stringTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "numberTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "boolTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "idx_service", Expression: "serviceName", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_name", Expression: "name", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_kind", Expression: "kind", Type: "minmax", Granularity: 4},
						{Name: "idx_duration", Expression: "durationNano", Type: "minmax", Granularity: 1},
						{Name: "idx_hasError", Expression: "hasError", Type: "set(2)", Granularity: 1},
						{Name: "idx_httpRoute", Expression: "httpRoute", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpUrl", Expression: "httpUrl", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpHost", Expression: "httpHost", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_httpMethod", Expression: "httpMethod", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_timestamp", Expression: "timestamp", Type: "minmax", Granularity: 1},
						{Name: "idx_rpcMethod", Expression: "rpcMethod", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_responseStatusCode", Expression: "responseStatusCode", Type: "set(0)", Granularity: 1},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp)",
						OrderBy:     "(durationNano, timestamp)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{
								Name:  "index_granularity",
								Value: "8192",
							},
							{
								Name:  "ttl_only_drop_parts",
								Value: "1",
							},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "durationSort",
				},
			},
		},
		{
			MigrationID: 4,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "durationSortMV",
					DestTable: "durationSort",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
						{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
						{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
						{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "stringTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "numberTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "boolTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Query: `SELECT
    timestamp,
    traceID,
    spanID,
    parentSpanID,
    serviceName,
    name,
    kind,
    durationNano,
    statusCode,
    httpMethod,
    httpUrl,
    httpRoute,
    httpHost,
    hasError,
    rpcSystem,
    rpcService,
    rpcMethod,
    responseStatusCode,
    stringTagMap,
    numberTagMap,
    boolTagMap,
    isRemote,
    statusMessage,
    statusCodeString,
    spanKind
FROM signoz_traces.signoz_index_v2
ORDER BY
    durationNano ASC,
    timestamp ASC`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "durationSortMV",
				},
			},
		},
		{
			MigrationID: 5,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_durationSort",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "parentSpanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "kind", Type: ColumnTypeInt8, Codec: "T64, ZSTD(1)"},
						{Name: "durationNano", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
						{Name: "statusCode", Type: ColumnTypeInt16, Codec: "T64, ZSTD(1)"},
						{Name: "httpMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpUrl", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpRoute", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "httpHost", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "hasError", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "rpcSystem", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcService", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "rpcMethod", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "responseStatusCode", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "stringTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "numberTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "boolTagMap", Type: MapColumnType{ColumnTypeString, ColumnTypeBool}, Codec: "ZSTD(1)"},
						{Name: "isRemote", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "statusMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "statusCodeString", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "spanKind", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "durationSort",
						ShardingKey: "cityHash64(traceID)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_durationSort",
				},
			},
		},
		{
			MigrationID: 6,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_error_index_v2",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "errorID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "groupID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "exceptionType", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "exceptionMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exceptionStacktrace", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exceptionEscaped", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "resourceTagsMap", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "idx_error_id", Expression: "errorID", Type: "bloom_filter", Granularity: 4},
						{Name: "idx_resourceTagsMapKeys", Expression: "mapKeys(resourceTagsMap)", Type: "bloom_filter(0.01)", Granularity: 64},
						{Name: "idx_resourceTagsMapValues", Expression: "mapValues(resourceTagsMap)", Type: "bloom_filter(0.01)", Granularity: 64},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp)",
						OrderBy:     "(timestamp, groupID)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_error_index_v2",
				},
			},
		},
		{
			MigrationID: 7,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_error_index_v2",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "errorID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "groupID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "spanID", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "exceptionType", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "exceptionMessage", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exceptionStacktrace", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exceptionEscaped", Type: ColumnTypeBool, Codec: "T64, ZSTD(1)"},
						{Name: "resourceTagsMap", Type: MapColumnType{LowCardinalityColumnType{ColumnTypeString}, ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "signoz_error_index_v2",
						ShardingKey: "cityHash64(groupID)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_error_index_v2",
				},
			},
		},
		{
			MigrationID: 8,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_spans",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "model", Type: ColumnTypeString, Codec: "ZSTD(9)"},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp)",
						OrderBy:     "(traceID)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "1024"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "signoz_spans",
				},
			},
		},
		{
			MigrationID: 9,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_spans",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "traceID", Type: FixedStringColumnType{Length: 32}, Codec: "ZSTD(1)"},
						{Name: "model", Type: ColumnTypeString, Codec: "ZSTD(9)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "signoz_spans",
						ShardingKey: "cityHash64(traceID)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_signoz_spans",
				},
			},
		},
		{
			MigrationID: 10,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "dependency_graph_minutes_v2",
					Columns: []Column{
						{Name: "src", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dest", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "duration_quantiles_state", Type: AggregateFunction{FunctionName: "quantiles(0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Default"},
						{Name: "error_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "total_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, LZ4"},
						{Name: "deployment_environment", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_cluster_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_namespace_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Engine: AggregatingMergeTree{MergeTree{
						PartitionBy: "toDate(timestamp)",
						OrderBy:     "(timestamp, src, dest, deployment_environment, k8s_cluster_name, k8s_namespace_name)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
						},
					}},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "dependency_graph_minutes_v2",
				},
			},
		},
		{
			MigrationID: 11,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "dependency_graph_minutes_db_calls_mv_v2",
					DestTable: "dependency_graph_minutes_v2",
					Columns: []Column{
						{Name: "src", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dest", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "duration_quantiles_state", Type: AggregateFunction{FunctionName: "quantiles(0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Default"},
						{Name: "error_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "total_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, LZ4"},
						{Name: "deployment_environment", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_cluster_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_namespace_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT
    serviceName AS src,
    dbSystem AS dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) AS duration_quantiles_state,
    countIf(statusCode = 2) AS error_count,
    count(*) AS total_count,
    toStartOfMinute(timestamp) AS timestamp,
    resourceTagsMap['deployment.environment'] AS deployment_environment,
    resourceTagsMap['k8s.cluster.name'] AS k8s_cluster_name,
    resourceTagsMap['k8s.namespace.name'] AS k8s_namespace_name
FROM signoz_traces.signoz_index_v2
WHERE (dest != '') AND (kind != 2)
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
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "dependency_graph_minutes_db_calls_mv_v2",
				},
			},
		},
		{
			MigrationID: 12,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "dependency_graph_minutes_messaging_calls_mv_v2",
					DestTable: "dependency_graph_minutes_v2",
					Columns: []Column{
						{Name: "src", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dest", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "duration_quantiles_state", Type: AggregateFunction{FunctionName: "quantiles(0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Default"},
						{Name: "error_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "total_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, LZ4"},
						{Name: "deployment_environment", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_cluster_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_namespace_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT
    serviceName AS src,
    msgSystem AS dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(durationNano)) AS duration_quantiles_state,
    countIf(statusCode = 2) AS error_count,
    count(*) AS total_count,
    toStartOfMinute(timestamp) AS timestamp,
    resourceTagsMap['deployment.environment'] AS deployment_environment,
    resourceTagsMap['k8s.cluster.name'] AS k8s_cluster_name,
    resourceTagsMap['k8s.namespace.name'] AS k8s_namespace_name
FROM signoz_traces.signoz_index_v2
WHERE (dest != '') AND (kind != 2)
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
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "dependency_graph_minutes_messaging_calls_mv_v2",
				},
			},
		},
		{
			MigrationID: 13,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "dependency_graph_minutes_service_calls_mv_v2",
					DestTable: "dependency_graph_minutes_v2",
					Columns: []Column{
						{Name: "src", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dest", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "duration_quantiles_state", Type: AggregateFunction{FunctionName: "quantiles(0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Default"},
						{Name: "error_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "total_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, LZ4"},
						{Name: "deployment_environment", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_cluster_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_namespace_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT
    A.serviceName AS src,
    B.serviceName AS dest,
    quantilesState(0.5, 0.75, 0.9, 0.95, 0.99)(toFloat64(B.durationNano)) AS duration_quantiles_state,
    countIf(B.statusCode = 2) AS error_count,
    count(*) AS total_count,
    toStartOfMinute(B.timestamp) AS timestamp,
    B.resourceTagsMap['deployment.environment'] AS deployment_environment,
    B.resourceTagsMap['k8s.cluster.name'] AS k8s_cluster_name,
    B.resourceTagsMap['k8s.namespace.name'] AS k8s_namespace_name
FROM signoz_traces.signoz_index_v2 AS A, signoz_traces.signoz_index_v2 AS B
WHERE (A.serviceName != B.serviceName) AND (A.spanID = B.parentSpanID)
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
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "dependency_graph_minutes_service_calls_mv_v2",
				},
			},
		},
		{
			MigrationID: 14,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_dependency_graph_minutes_v2",
					Columns: []Column{
						{Name: "src", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "dest", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "duration_quantiles_state", Type: AggregateFunction{FunctionName: "quantiles(0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Default"},
						{Name: "error_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "total_count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "T64, ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, LZ4"},
						{Name: "deployment_environment", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_cluster_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "k8s_namespace_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "dependency_graph_minutes_v2",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_dependency_graph_minutes_v2",
				},
			},
		},
		{
			MigrationID: 15,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "usage_explorer",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "service_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "count", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					},
					Engine: SummingMergeTree{MergeTree{
						PartitionBy: "toDate(timestamp)",
						OrderBy:     "(timestamp, service_name)",
						TTL:         "toDateTime(timestamp) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					}},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "usage_explorer",
				},
			},
		},
		{
			MigrationID: 16,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "usage_explorer_mv",
					DestTable: "usage_explorer",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "service_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "count", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					},
					Query: `SELECT
    toStartOfHour(timestamp) AS timestamp,
    serviceName AS service_name,
    count() AS count
FROM signoz_traces.signoz_index_v2
GROUP BY
    timestamp,
    serviceName`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "usage_explorer_mv",
				},
			},
		},
		{
			MigrationID: 17,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_usage_explorer",
					Columns: []Column{
						{Name: "timestamp", Type: DateTime64ColumnType{Precision: 9}, Codec: "DoubleDelta, LZ4"},
						{Name: "service_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "count", Type: ColumnTypeUInt64, Codec: "T64, ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "usage_explorer",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_usage_explorer",
				},
			},
		},
		{
			MigrationID: 18,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "top_level_operations",
					Columns: []Column{
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "time", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							OrderBy: "(serviceName, name)",
							TTL:     "time + toIntervalMonth(1)",
							Settings: TableSettings{
								{Name: "index_granularity", Value: "8192"},
							},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "top_level_operations",
				},
			},
		},
		{
			MigrationID: 19,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_top_level_operations",
					Columns: []Column{
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "time", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "top_level_operations",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_top_level_operations",
				},
			},
		},
		{
			MigrationID: 20,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "root_operations",
					DestTable: "top_level_operations",
					Columns: []Column{
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT DISTINCT
    name,
    serviceName
FROM signoz_traces.signoz_index_v2
WHERE parentSpanID = ''`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "root_operations",
				},
			},
		},
		{
			MigrationID: 21,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_traces",
					ViewName:  "sub_root_operations",
					DestTable: "top_level_operations",
					Columns: []Column{
						{Name: "name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "serviceName", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT DISTINCT
    name,
    serviceName
FROM signoz_traces.signoz_index_v2 AS A, signoz_traces.signoz_index_v2 AS B
WHERE (A.serviceName != B.serviceName) AND (A.parentSpanID = B.spanID)`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "sub_root_operations",
				},
			},
		},
		{
			MigrationID: 22,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "span_attributes",
					Columns: []Column{
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "dataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'float64' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "stringTagValue", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "float64TagValue", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "isColumn", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							OrderBy: "(tagKey, tagType, dataType, stringTagValue, float64TagValue, isColumn)",
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
					Database: "signoz_traces",
					Table:    "span_attributes",
				},
			},
		},
		{
			MigrationID: 23,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_span_attributes",
					Columns: []Column{
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "dataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'float64' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "stringTagValue", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "float64TagValue", Type: NullableColumnType{ColumnTypeFloat64}, Codec: "ZSTD(1)"},
						{Name: "isColumn", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "span_attributes",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_span_attributes",
				},
			},
		},
		{
			MigrationID: 24,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "span_attributes_keys",
					Columns: []Column{
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "dataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'float64' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "isColumn", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							OrderBy: "(tagKey, tagType, dataType, isColumn)",
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "span_attributes_keys",
				},
			},
		},
		{
			MigrationID: 25,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_span_attributes_keys",
					Columns: []Column{
						{Name: "tagKey", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
						{Name: "tagType", Type: EnumerationColumnType{Values: []string{"'tag' = 1", "'resource' = 2"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "dataType", Type: EnumerationColumnType{Values: []string{"'string' = 1", "'bool' = 2", "'float64' = 3"}, Size: 8}, Codec: "ZSTD(1)"},
						{Name: "isColumn", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "span_attributes_keys",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_span_attributes_keys",
				},
			},
		},
		{
			MigrationID: 26,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
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
					Database: "signoz_traces",
					Table:    "usage",
				},
			},
		},
		{
			MigrationID: 27,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_traces",
					Table:    "distributed_usage",
					Columns: []Column{
						{Name: "tenant", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "collector_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exporter_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "data", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_traces",
						Table:       "usage",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
		},
	}
)
