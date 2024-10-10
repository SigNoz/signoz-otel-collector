package schemamigrator

var MetricsMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_6hrs",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_6hrs",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_6hrs",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1day",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1day",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1day",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1week",
				Column: Column{
					Name: "attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1week",
				Column: Column{
					Name: "scope_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1week",
				Column: Column{
					Name: "resource_attrs",
					Type: MapColumnType{
						KeyType:   LowCardinalityColumnType{ColumnTypeString},
						ValueType: ColumnTypeString,
					},
					Codec:   "ZSTD(1)",
					Default: "map()",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "metadata",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "description", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					{Name: "attr_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_datatype", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "first_reported_unix_milli", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "last_reported_unix_milli", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						OrderBy:     "(temporality, metric_name, attr_name, attr_type, attr_datatype, attr_string_value)",
						PartitionBy: "toDate(last_reported_unix_milli / 1000)",
						TTL:         "toDateTime(last_reported_unix_milli / 1000) + toIntervalSecond(2592000)",
						Settings: []TableSetting{
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_metadata",
				Columns: []Column{
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "description", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					{Name: "attr_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_datatype", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "attr_string_value", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "first_reported_unix_milli", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					{Name: "last_reported_unix_milli", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Cluster:     "cluster",
					Database:    "signoz_metrics",
					Table:       "metadata",
					ShardingKey: "rand()",
				},
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "time_series_v4_6hrs_mv_separate_attrs",
				DestTable: "time_series_v4_6hrs",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
				},
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
labels,
attrs,
scope_attrs,
resource_attrs
FROM signoz_metrics.time_series_v4`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "time_series_v4_1day_mv_separate_attrs",
				DestTable: "time_series_v4_1day",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
				},
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
labels,
attrs,
scope_attrs,
resource_attrs
FROM signoz_metrics.time_series_v4_6hrs`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "time_series_v4_1week_mv_separate_attrs",
				DestTable: "time_series_v4_1week",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)"},
				},
				Query: `SELECT
env,
temporality,
metric_name,
description,
unit,
type,
is_monotonic,
fingerprint,
floor(unix_milli / 604800000) * 604800000 AS unix_milli,
labels,
attrs,
scope_attrs,
resource_attrs
FROM signoz_metrics.time_series_v4_1day`,
			},
		},
	},
}
