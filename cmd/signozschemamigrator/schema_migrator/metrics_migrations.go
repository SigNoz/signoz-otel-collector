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
	{
		MigrationID: 1001,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_6hrs",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1day",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_1week",
				Column: Column{
					Name:    "__normalized",
					Type:    ColumnTypeBool,
					Codec:   "ZSTD(1)",
					Default: "true",
				},
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_6hrs_mv",
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
							__normalized
						FROM signoz_metrics.time_series_v4`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1day_mv",
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
							__normalized
						FROM signoz_metrics.time_series_v4_6hrs`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1week_mv",
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
							__normalized
						FROM signoz_metrics.time_series_v4_1day`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_6hrs_mv_separate_attrs",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1day_mv_separate_attrs",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4_6hrs`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1week_mv_separate_attrs",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4_1day`,
			},
		},
		// no need for down items, and there is a default value for the column
		// so it's a safe migration without any down migration
	},
	{
		MigrationID: 1002,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "updated_metadata",
				Columns: []Column{
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "created_at", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: MergeTree{
					OrderBy: "(metric_name)",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_updated_metadata",
				Columns: []Column{
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "created_at", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics",
					Table:       "updated_metadata",
					ShardingKey: "cityHash64(metric_name)",
				},
			},
		},
	},
	{
		MigrationID: 1003,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "samples_v4",
				Column: Column{
					Name:    "flags",
					Type:    ColumnTypeUInt32,
					Codec:   "ZSTD(1)",
					Default: "0",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_samples_v4",
				Column: Column{
					Name:    "flags",
					Type:    ColumnTypeUInt32,
					Codec:   "ZSTD(1)",
					Default: "0",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "exp_hist",
				Column: Column{
					Name:    "flags",
					Type:    ColumnTypeUInt32,
					Codec:   "ZSTD(1)",
					Default: "0",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_exp_hist",
				Column: Column{
					Name:    "flags",
					Type:    ColumnTypeUInt32,
					Codec:   "ZSTD(1)",
					Default: "0",
				},
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "samples_v4_agg_5m_mv",
				Query: `SELECT
							env,
							temporality,
							metric_name,
							fingerprint,
							intDiv(unix_milli, 300000) * 300000 as unix_milli,
							anyLast(value) as last,
							min(value) as min,
							max(value) as max,
							sum(value) as sum,
							count(*) as count
						FROM signoz_metrics.samples_v4
						WHERE bitAnd(flags, 1) = 0
						GROUP BY
							env,
							temporality,
							metric_name,
							fingerprint,
							unix_milli;`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "samples_v4_agg_30m_mv",
				Query: `SELECT
							env,
							temporality,
							metric_name,
							fingerprint,
							intDiv(unix_milli, 1800000) * 1800000 AS unix_milli,
							anyLast(last) AS last,
							min(min) AS min,
							max(max) AS max,
							sum(sum) AS sum,
							sum(count) AS count
						FROM signoz_metrics.samples_v4_agg_5m
						GROUP BY
							env,
							temporality,
							metric_name,
							fingerprint,
							unix_milli;`,
			},
		},
	},
	{
		MigrationID: 1004,
		UpItems: []Operation{
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_6hrs_mv",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1day_mv",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4_6hrs`,
			},
			ModifyQueryMaterializedViewOperation{
				Database: "signoz_metrics",
				ViewName: "time_series_v4_1week_mv",
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
							resource_attrs,
							__normalized
						FROM signoz_metrics.time_series_v4_1day`,
			},
		},
	},
	{
		MigrationID: 1005,
		UpItems: []Operation{
			DropTableOperation{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs_mv_separate_attrs",
			},
			DropTableOperation{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day_mv_separate_attrs",
			},
			DropTableOperation{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week_mv_separate_attrs",
			},
		},
	},
	{
		MigrationID: 1006,
		UpItems: []Operation{
			AlterTableDropIndex{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Index: Index{
					Name: "idx_labels",
				},
			},
			AlterTableDropIndex{
				Database: "signoz_metrics",
				Table:    "time_series_v4_6hrs",
				Index: Index{
					Name: "idx_labels",
				},
			},
			AlterTableDropIndex{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1day",
				Index: Index{
					Name: "idx_labels",
				},
			},
			AlterTableDropIndex{
				Database: "signoz_metrics",
				Table:    "time_series_v4_1week",
				Index: Index{
					Name: "idx_labels",
				},
			},
		},
	},
	{
		MigrationID: 1007,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "samples_v4",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_samples_v4",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "time_series_v4",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "exp_hist",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_metrics",
				Table:    "distributed_exp_hist",
				Column: Column{
					Name:  "inserted_at_unix_milli",
					Type:  ColumnTypeInt64,
					Codec: "ZSTD(1)",
				},
			},
		},
	},
	{
		// Cardinality control: buffer tables are the universal landing target.
		// Incremental MVs feed long-retention tables for unruled rows
		// (reduced_fingerprint = 0); refreshable MVs aggregate ruled series into
		// the 60s reduced tables. Rules live in metric_reduction_rules.
		MigrationID: 1008,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "samples_v4_buffer",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)", Default: "0"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "false"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "ZSTD(1)", Default: "0"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Indexes: []Index{
					// minmax index lets each refresh prune to the small fresh parts
					{Name: "idx_unix_milli", Expression: "unix_milli", Type: "minmax", Granularity: 1},
				},
				Engine: MergeTree{
					PartitionBy: "toDate(unix_milli / 1000)",
					OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
					// with ttl_only_drop_parts and daily partitions effective retention is 24-48h
					TTL: "toDateTime(unix_milli / 1000) + toIntervalSecond(86400)",
					Settings: TableSettings{
						{Name: "index_granularity", Value: "8192"},
						{Name: "ttl_only_drop_parts", Value: "1"},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_samples_v4_buffer",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)", Default: "0"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "false"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "ZSTD(1)", Default: "0"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database: "signoz_metrics",
					Table:    "samples_v4_buffer",
					// all series of a reduced group must land on one shard so the per-shard refreshable MVs see the whole group
					ShardingKey: "cityHash64(env, temporality, metric_name, if(reduced_fingerprint != 0, reduced_fingerprint, fingerprint))",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "time_series_v4_buffer",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)", Default: "0"},
					{Name: "is_reduced", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "false"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						// is_reduced is part of the dedup identity so a self-reducing series keeps both its raw and reduced rows
						OrderBy: "(env, temporality, metric_name, fingerprint, unix_milli, is_reduced)",
						TTL:     "toDateTime(unix_milli / 1000) + toIntervalSecond(86400)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_buffer",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
					{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)", Default: "0"},
					{Name: "is_reduced", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "false"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database: "signoz_metrics",
					Table:    "time_series_v4_buffer",
					// same expression as distributed_samples_v4_buffer so series rows stay co-located with their samples
					ShardingKey: "cityHash64(env, temporality, metric_name, if(reduced_fingerprint != 0, reduced_fingerprint, fingerprint))",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "time_series_v4_reduced",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
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
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_time_series_v4_reduced",
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
					{Name: "attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "scope_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "resource_attrs", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}, Codec: "ZSTD(1)", Default: "map()"},
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database: "signoz_metrics",
					Table:    "time_series_v4_reduced",
					// the fingerprint column holds the reduced fingerprint, matching the reduced-sample shard placement
					ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "samples_v4_reduced_last_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "sum_values", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "count_samples", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
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
					// buckets are recomputed and appended across refreshes; merges keep the latest, reads use argMax(computed_at)
					Version: "computed_at",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_samples_v4_reduced_last_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum_last", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "min", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "max", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "sum_values", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "count_samples", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics",
					Table:       "samples_v4_reduced_last_60s",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "samples_v4_reduced_sum_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "count_samples", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
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
					Version: "computed_at",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_samples_v4_reduced_sum_60s",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "sum", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "count_series", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "count_samples", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
					{Name: "computed_at", Type: DateTimeColumnType{}, Default: "now()", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics",
					Table:       "samples_v4_reduced_sum_60s",
					ShardingKey: "cityHash64(env, temporality, metric_name, reduced_fingerprint)",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "metric_reduction_rules",
				Columns: []Column{
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "drop_labels", Type: ArrayColumnType{ElementType: LowCardinalityColumnType{ColumnTypeString}}, Codec: "ZSTD(1)"},
					{Name: "effective_from_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
					{Name: "deleted", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 3}, Default: "now64(3)", Codec: "ZSTD(1)"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						OrderBy: "metric_name",
					},
					Version: "updated_at",
				},
			},
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_metric_reduction_rules",
				Columns: []Column{
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}, Codec: "ZSTD(1)"},
					{Name: "drop_labels", Type: ArrayColumnType{ElementType: LowCardinalityColumnType{ColumnTypeString}}, Codec: "ZSTD(1)"},
					{Name: "effective_from_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
					{Name: "deleted", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 3}, Default: "now64(3)", Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metrics",
					Table:       "metric_reduction_rules",
					ShardingKey: "cityHash64(metric_name)",
				},
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "samples_v4_mv",
				DestTable: "samples_v4",
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
					{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					{Name: "flags", Type: ColumnTypeUInt32, Codec: "ZSTD(1)", Default: "0"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
				},
				Query: `SELECT
							env,
							temporality,
							metric_name,
							fingerprint,
							unix_milli,
							value,
							flags,
							inserted_at_unix_milli
						FROM signoz_metrics.samples_v4_buffer
						WHERE reduced_fingerprint = 0`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "time_series_v4_mv",
				DestTable: "time_series_v4",
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
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
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
							unix_milli,
							labels,
							attrs,
							scope_attrs,
							resource_attrs,
							__normalized,
							inserted_at_unix_milli
						FROM signoz_metrics.time_series_v4_buffer
						WHERE is_reduced = false AND reduced_fingerprint = 0`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metrics",
				ViewName:  "time_series_v4_reduced_mv",
				DestTable: "time_series_v4_reduced",
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
					{Name: "__normalized", Type: ColumnTypeBool, Codec: "ZSTD(1)", Default: "true"},
					{Name: "inserted_at_unix_milli", Type: ColumnTypeInt64, Codec: "ZSTD(1)"},
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
							unix_milli,
							labels,
							attrs,
							scope_attrs,
							resource_attrs,
							__normalized,
							inserted_at_unix_milli
						FROM signoz_metrics.time_series_v4_buffer
						WHERE is_reduced = true`,
			},
			CreateRefreshableMaterializedViewOperation{
				Database:     "signoz_metrics",
				ViewName:     "samples_v4_reduced_last_60s_mv",
				DestTable:    "samples_v4_reduced_last_60s",
				RefreshEvery: "1 MINUTE",
				RandomizeFor: "10 SECOND",
				Append:       true,
				Empty:        true,
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64},
					{Name: "unix_milli", Type: ColumnTypeInt64},
					{Name: "sum_last", Type: ColumnTypeFloat64},
					{Name: "min", Type: ColumnTypeFloat64},
					{Name: "max", Type: ColumnTypeFloat64},
					{Name: "sum_values", Type: ColumnTypeFloat64},
					{Name: "count_series", Type: ColumnTypeUInt64},
					{Name: "count_samples", Type: ColumnTypeUInt64},
					{Name: "computed_at", Type: DateTimeColumnType{}},
				},
				// gauges and non-monotonic cumulative sums (UpDownCounters): last value
				// per series per 60s bucket, then aggregated across series. only whole
				// buckets are emitted (scan reaches now-11m, emit bucket_unix_milli >=
				// now-10m) so the latest computed_at row is never a truncated sliver.
				// the bucket alias must NOT be named unix_milli: it would shadow the
				// source column inside argMax/min/max and break the aggregation.
				Query: `SELECT
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli AS unix_milli,
							sum(last) AS sum_last,
							min(min_value) AS min,
							max(max_value) AS max,
							sum(sum_value) AS sum_values,
							count() AS count_series,
							sum(num_values) AS count_samples,
							now() AS computed_at
						FROM
						(
							SELECT
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								intDiv(unix_milli, 60000) * 60000 AS bucket_unix_milli,
								argMax(value, unix_milli) AS last,
								min(value) AS min_value,
								max(value) AS max_value,
								sum(value) AS sum_value,
								count(value) AS num_values
							FROM signoz_metrics.samples_v4_buffer
							PREWHERE reduced_fingerprint != 0
							WHERE (bitAnd(flags, 1) = 0)
								AND ((temporality = 'Unspecified') OR ((temporality = 'Cumulative') AND (is_monotonic = false)))
								AND (unix_milli < (toUnixTimestamp(now() - toIntervalSecond(120)) * 1000))
								AND (unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(11)) * 1000))
							GROUP BY
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								bucket_unix_milli
						)
						WHERE bucket_unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(10)) * 1000)
						GROUP BY
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli`,
			},
			CreateRefreshableMaterializedViewOperation{
				Database:     "signoz_metrics",
				ViewName:     "samples_v4_reduced_sum_60s_delta_mv",
				DestTable:    "samples_v4_reduced_sum_60s",
				RefreshEvery: "1 MINUTE",
				RandomizeFor: "10 SECOND",
				Append:       true,
				Empty:        true,
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64},
					{Name: "unix_milli", Type: ColumnTypeInt64},
					{Name: "sum", Type: ColumnTypeFloat64},
					{Name: "count_series", Type: ColumnTypeUInt64},
					{Name: "count_samples", Type: ColumnTypeUInt64},
					{Name: "computed_at", Type: DateTimeColumnType{}},
				},
				// delta sums commute: per-series sum within the bucket, then sum across
				// series. only whole buckets are emitted (scan reaches now-11m, emit
				// bucket_unix_milli >= now-10m) so the latest computed_at row is never a
				// truncated sliver.
				Query: `SELECT
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli AS unix_milli,
							sum(series_sum) AS sum,
							count() AS count_series,
							sum(num_values) AS count_samples,
							now() AS computed_at
						FROM
						(
							SELECT
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								intDiv(unix_milli, 60000) * 60000 AS bucket_unix_milli,
								sum(value) AS series_sum,
								count(value) AS num_values
							FROM signoz_metrics.samples_v4_buffer
							PREWHERE reduced_fingerprint != 0
							WHERE (bitAnd(flags, 1) = 0)
								AND (temporality = 'Delta')
								AND (unix_milli < (toUnixTimestamp(now() - toIntervalSecond(120)) * 1000))
								AND (unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(11)) * 1000))
							GROUP BY
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								bucket_unix_milli
						)
						WHERE bucket_unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(10)) * 1000)
						GROUP BY
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli`,
			},
			CreateRefreshableMaterializedViewOperation{
				Database:     "signoz_metrics",
				ViewName:     "samples_v4_reduced_sum_60s_cumulative_mv",
				DestTable:    "samples_v4_reduced_sum_60s",
				RefreshEvery: "1 MINUTE",
				RandomizeFor: "10 SECOND",
				Append:       true,
				Empty:        true,
				Columns: []Column{
					{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
					{Name: "reduced_fingerprint", Type: ColumnTypeUInt64},
					{Name: "unix_milli", Type: ColumnTypeInt64},
					{Name: "sum", Type: ColumnTypeFloat64},
					{Name: "count_series", Type: ColumnTypeUInt64},
					{Name: "count_samples", Type: ColumnTypeUInt64},
					{Name: "computed_at", Type: DateTimeColumnType{}},
				},
				// cumulative counters reduced to per-bucket increments with per-point
				// reset detection (a value drop counts the post-reset value as the
				// increment). the scan reaches one minute further back to supply the
				// previous point at the window edge; a series' first point yields no
				// increment (Prometheus increase() semantics).
				Query: `SELECT
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli AS unix_milli,
							sum(series_increment) AS sum,
							count() AS count_series,
							sum(num_values) AS count_samples,
							now() AS computed_at
						FROM
						(
							SELECT
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								bucket_unix_milli,
								sum(increment) AS series_increment,
								count() AS num_values
							FROM
							(
								SELECT
									env,
									temporality,
									metric_name,
									reduced_fingerprint,
									fingerprint,
									intDiv(unix_milli, 60000) * 60000 AS bucket_unix_milli,
									multiIf(
										row_number() OVER rate_window = 1, nan,
										value < lagInFrame(value, 1) OVER rate_window, value,
										value - lagInFrame(value, 1) OVER rate_window
									) AS increment
								FROM signoz_metrics.samples_v4_buffer
								PREWHERE reduced_fingerprint != 0
								WHERE (bitAnd(flags, 1) = 0)
									AND ((temporality = 'Cumulative') AND (is_monotonic = true))
									AND (unix_milli < (toUnixTimestamp(now() - toIntervalSecond(120)) * 1000))
									AND (unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(6)) * 1000))
								WINDOW rate_window AS (PARTITION BY env, temporality, metric_name, fingerprint ORDER BY unix_milli)
							)
							WHERE (isNaN(increment) = 0)
								AND (bucket_unix_milli >= (toUnixTimestamp(now() - toIntervalMinute(5)) * 1000))
							GROUP BY
								env,
								temporality,
								metric_name,
								reduced_fingerprint,
								fingerprint,
								bucket_unix_milli
						)
						GROUP BY
							env,
							temporality,
							metric_name,
							reduced_fingerprint,
							bucket_unix_milli`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_reduced_sum_60s_cumulative_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_reduced_sum_60s_delta_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_reduced_last_60s_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "time_series_v4_reduced_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "time_series_v4_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_mv"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_metric_reduction_rules"},
			DropTableOperation{Database: "signoz_metrics", Table: "metric_reduction_rules"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_samples_v4_reduced_sum_60s"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_reduced_sum_60s"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_samples_v4_reduced_last_60s"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_reduced_last_60s"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_time_series_v4_reduced"},
			DropTableOperation{Database: "signoz_metrics", Table: "time_series_v4_reduced"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_time_series_v4_buffer"},
			DropTableOperation{Database: "signoz_metrics", Table: "time_series_v4_buffer"},
			DropTableOperation{Database: "signoz_metrics", Table: "distributed_samples_v4_buffer"},
			DropTableOperation{Database: "signoz_metrics", Table: "samples_v4_buffer"},
		},
	},
}
