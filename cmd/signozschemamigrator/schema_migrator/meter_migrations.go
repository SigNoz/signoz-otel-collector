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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Gorilla, ZSTD(1)"},
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
				Database: "signoz_metrics",
				Table:    "samples_v4",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "Gorilla, ZSTD(1)"},
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
}
