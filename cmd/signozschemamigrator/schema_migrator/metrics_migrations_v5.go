package schemamigrator

var MetricsV5Migrations = []SchemaMigrationRecord{
	{
		MigrationID: 29,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Default: "0", Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Default: "0", Codec: "ZSTD(1)"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: MergeTree{
					PartitionBy: "toDate(unix_milli / 1000)",
					OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
					TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5"},
		},
	},
	{
		MigrationID: 30,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Default: "0", Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Default: "0", Codec: "ZSTD(1)"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5"},
		},
	},
	{
		MigrationID: 31,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_agg_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_agg_5m"},
		},
	},
	{
		MigrationID: 32,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_agg_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_agg_5m",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_agg_5m"},
		},
	},
	{
		MigrationID: 33,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_agg_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_agg_30m"},
		},
	},
	{
		MigrationID: 34,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_agg_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_agg_30m",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_agg_30m"},
		},
	},
	{
		MigrationID: 35,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "samples_v5_agg_5m_mv",
				DestTable: "samples_v5_agg_5m",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    anyLast(reduced_fingerprint) as reduced_fingerprint,
    intDiv(unix_milli, 300000) * 300000 as unix_milli,
    anyLast(value) as last,
    min(value) as min,
    max(value) as max,
    sum(value) as sum,
    count(*) as count
FROM signoz_metrics_v2.samples_v5
WHERE bitAnd(flags, 1) = 0
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_agg_5m_mv"},
		},
	},
	{
		MigrationID: 36,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "samples_v5_agg_30m_mv",
				DestTable: "samples_v5_agg_30m",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    anyLast(reduced_fingerprint) as reduced_fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 as unix_milli,
    anyLast(last) as last,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics_v2.samples_v5_agg_5m
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_agg_30m_mv"},
		},
	},

	// reduced but incremental views
	{
		MigrationID: 37,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_agg_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_60s"},
		},
	},
	{
		MigrationID: 38,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_agg_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_agg_60s",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_agg_60s"},
		},
	},
	{
		MigrationID: 39,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_agg_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_5m"},
		},
	},
	{
		MigrationID: 40,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_agg_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_agg_5m",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_agg_5m"},
		},
	},
	{
		MigrationID: 41,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_agg_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_30m"},
		},
	},
	{
		MigrationID: 42,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_agg_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_agg_30m",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_agg_30m"},
		},
	},
	{
		MigrationID: 43,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "samples_v5_reduced_agg_60s_mv",
				DestTable: "samples_v5_reduced_agg_60s",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    intDiv(unix_milli, 60000) * 60000 as unix_milli,
    min(value) as min,
    max(value) as max,
    sum(value) as sum,
    count(*) as count
FROM signoz_metrics_v2.samples_v5
WHERE bitAnd(flags, 1) = 0 AND temporality = 'Delta' AND reduced_fingerprint != 0
GROUP BY
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_60s_mv"},
		},
	},
	{
		MigrationID: 44,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "samples_v5_reduced_agg_5m_mv",
				DestTable: "samples_v5_reduced_agg_5m",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    intDiv(unix_milli, 300000) * 300000 as unix_milli,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics_v2.samples_v5_reduced_agg_60s
GROUP BY
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_5m_mv"},
		},
	},
	{
		MigrationID: 45,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "samples_v5_reduced_agg_30m_mv",
				DestTable: "samples_v5_reduced_agg_30m",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 as unix_milli,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics_v2.samples_v5_reduced_agg_5m
GROUP BY
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_agg_30m_mv"},
		},
	},

	// Reduced refreshable views (spatial aggregates of per-series LAST).
	// Gauge + cumulative Counter/Hist
	// Doesn't cascade: each resolution is built from the per-fingerprint tier
	// at that resolution (raw / agg_5m / agg_30m). Version-less ReplacingMergeTree;
	// read-side argMax(col, computed_at).
	// Should there be version in ReplacingMergeTree?
	{
		MigrationID: 46,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_last_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_60s"},
		},
	},
	{
		MigrationID: 47,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_last_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_last_60s",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_last_60s"},
		},
	},
	{
		MigrationID: 48,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_last_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_5m"},
		},
	},
	{
		MigrationID: 49,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_last_5m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_last_5m",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_last_5m"},
		},
	},
	{
		MigrationID: 50,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "samples_v5_reduced_last_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, reduced_fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_30m"},
		},
	},
	{
		MigrationID: 51,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_samples_v5_reduced_last_30m",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "samples_v5_reduced_last_30m",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_samples_v5_reduced_last_30m"},
		},
	},
	{
		MigrationID: 52,
		UpItems: []Operation{
			CreateRefreshableMaterializedViewOperation{
				Database:             "signoz_metrics_v2",
				ViewName:             "samples_v5_reduced_last_60s_mv",
				DestTable:            "samples_v5_reduced_last_60s",
				RefreshInterval:      "1 MINUTE",
				RandomizeForInterval: "10 SECOND",
				Append:               true,
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli,
    sum(fp_last) as sum_last,
    min(fp_last) as min_last,
    max(fp_last) as max_last,
    count() as count_series,
    now() as computed_at
FROM (
    SELECT
        env,
        temporality,
        metric_name,
        reduced_fingerprint,
        fingerprint,
        intDiv(unix_milli, 60000) * 60000 as unix_milli,
        argMax(value, unix_milli) as fp_last
    FROM signoz_metrics_v2.samples_v5
    WHERE bitAnd(flags, 1) = 0 AND reduced_fingerprint != 0
      AND temporality IN ('Unspecified', 'Cumulative')
      AND unix_milli <  toUnixTimestamp(now() - INTERVAL 90 SECOND) * 1000
      AND unix_milli >= toUnixTimestamp(now() - INTERVAL 5 MINUTE) * 1000
    GROUP BY env, temporality, metric_name, reduced_fingerprint, fingerprint, unix_milli
)
GROUP BY env, temporality, metric_name, reduced_fingerprint, unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_60s_mv"},
		},
	},
	{
		MigrationID: 53,
		UpItems: []Operation{
			CreateRefreshableMaterializedViewOperation{
				Database:             "signoz_metrics_v2",
				ViewName:             "samples_v5_reduced_last_5m_mv",
				DestTable:            "samples_v5_reduced_last_5m",
				RefreshInterval:      "5 MINUTE",
				RandomizeForInterval: "30 SECOND",
				Append:               true,
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli,
    sum(fp_last) as sum_last,
    min(fp_last) as min_last,
    max(fp_last) as max_last,
    count() as count_series,
    now() as computed_at
FROM (
    SELECT
        env,
        temporality,
        metric_name,
        anyLast(reduced_fingerprint) as reduced_fingerprint,
        fingerprint,
        unix_milli,
        anyLast(last) as fp_last
    FROM signoz_metrics_v2.samples_v5_agg_5m
    WHERE temporality IN ('Unspecified', 'Cumulative')
      AND unix_milli <  toUnixTimestamp(now() - INTERVAL 10 MINUTE) * 1000
      AND unix_milli >= toUnixTimestamp(now() - INTERVAL 60 MINUTE) * 1000
    GROUP BY env, temporality, metric_name, fingerprint, unix_milli
)
GROUP BY env, temporality, metric_name, reduced_fingerprint, unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_5m_mv"},
		},
	},
	{
		MigrationID: 54,
		UpItems: []Operation{
			CreateRefreshableMaterializedViewOperation{
				Database:             "signoz_metrics_v2",
				ViewName:             "samples_v5_reduced_last_30m_mv",
				DestTable:            "samples_v5_reduced_last_30m",
				RefreshInterval:      "30 MINUTE",
				RandomizeForInterval: "1 MINUTE",
				Append:               true,
				Query: `SELECT
    env,
    temporality,
    metric_name,
    reduced_fingerprint,
    unix_milli,
    sum(fp_last) as sum_last,
    min(fp_last) as min_last,
    max(fp_last) as max_last,
    count() as count_series,
    now() as computed_at
FROM (
    SELECT
        env,
        temporality,
        metric_name,
        anyLast(reduced_fingerprint) as reduced_fingerprint,
        fingerprint,
        unix_milli,
        anyLast(last) as fp_last
    FROM signoz_metrics_v2.samples_v5_agg_30m
    WHERE temporality IN ('Unspecified', 'Cumulative')
      AND unix_milli <  toUnixTimestamp(now() - INTERVAL 60 MINUTE) * 1000
      AND unix_milli >= toUnixTimestamp(now() - INTERVAL 240 MINUTE) * 1000
    GROUP BY env, temporality, metric_name, fingerprint, unix_milli
)
GROUP BY env, temporality, metric_name, reduced_fingerprint, unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "samples_v5_reduced_last_30m_mv"},
		},
	},

	{
		MigrationID: 55,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "time_series_v5",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "time_series_v5"},
		},
	},
	{
		MigrationID: 56,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_time_series_v5",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "time_series_v5",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_time_series_v5"},
		},
	},
	{
		MigrationID: 57,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "time_series_v5_6hrs",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "time_series_v5_6hrs"},
		},
	},
	{
		MigrationID: 58,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_time_series_v5_6hrs",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "time_series_v5_6hrs",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_time_series_v5_6hrs"},
		},
	},
	{
		MigrationID: 59,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "time_series_v5_1day",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
						TTL:         "toDateTime(unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "time_series_v5_1day"},
		},
	},
	{
		MigrationID: 60,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics_v2",
				Table:    "distributed_time_series_v5_1day",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics_v2",
					Table:       "time_series_v5_1day",
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "distributed_time_series_v5_1day"},
		},
	},
	{
		MigrationID: 61,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "time_series_v5_6hrs_mv",
				DestTable: "time_series_v5_6hrs",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    floor(unix_milli / 21600000) * 21600000 AS unix_milli,
    labels
FROM signoz_metrics_v2.time_series_v5`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "time_series_v5_6hrs_mv"},
		},
	},
	{
		MigrationID: 62,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics_v2",
				ViewName:  "time_series_v5_1day_mv",
				DestTable: "time_series_v5_1day",
				Query: `SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    floor(unix_milli / 86400000) * 86400000 AS unix_milli,
    labels
FROM signoz_metrics_v2.time_series_v5_6hrs`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics_v2", Table: "time_series_v5_1day_mv"},
		},
	},
}
