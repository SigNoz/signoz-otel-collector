package schemamigrator

var MeterMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_meter",
				Table:    "samples",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
				},
				Engine: MergeTree{
					PartitionBy: "toYYYYMM(toDateTime(intDiv(unix_milli, 1000)))",
					OrderBy:     "(temporality, metric_name, fingerprint, toDayOfMonth(toDateTime(intDiv(unix_milli, 1000))))",
					TTL:         "toDateTime(intDiv(unix_milli, 1000)) + toIntervalYear(1)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_meter",
				Table:    "samples",
			},
		},
	},
	{
		MigrationID: 2,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_meter",
				Table:    "distributed_samples",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_meter",
					Table:       "samples",
					ShardingKey: "cityHash64(temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_meter",
				Table:    "distributed_samples",
			},
		},
	},
	{
		MigrationID: 3,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_meter",
				Table:    "samples_agg_1d",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toYYYYMM(toDateTime(intDiv(unix_milli, 1000)))",
						OrderBy:     "(temporality, metric_name, fingerprint, toDayOfMonth(toDateTime(intDiv(unix_milli, 1000))))",
						TTL:         "toDateTime(intDiv(unix_milli, 1000)) + toIntervalYear(1)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_meter",
				Table:    "samples_agg_1d",
			},
		},
	},
	{
		MigrationID: 4,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_meter",
				Table:    "distributed_samples_agg_1d",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_meter",
					Table:       "samples_agg_1d",
					ShardingKey: "cityHash64(temporality, metric_name, fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_meter",
				Table:    "distributed_samples_agg_1d",
			},
		},
	},
	{
		MigrationID: 5,
		UpItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_meter",
				ViewName:  "samples_agg_1d_mv",
				DestTable: "samples_agg_1d",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
					{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Query: `SELECT
							temporality,
							metric_name,
							description,
							unit,
							type,
							is_monotonic,
							labels,
							fingerprint,
							intDiv(unix_milli,86400000) * 86400000 as unix_milli,
							anyLast(value) as last,
							min(value) as min,
							max(value) as max,
							sum(value) as sum,
							count(*) as count
						FROM signoz_meter.samples
						GROUP BY
							temporality,
							metric_name,
							fingerprint,
							description,
							unit,
							type,
							is_monotonic,
							labels,
							unix_milli;`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_meter",
				Table:    "samples_agg_1d_mv",
			},
		},
	},
}
