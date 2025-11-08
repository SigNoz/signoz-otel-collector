package schemamigrator

var (
	SquashedMetricsMigrations = []SchemaMigrationRecord{
		{
			MigrationID: 1,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v2",
					Columns: []Column{
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "timestamp_ms", Type: ColumnTypeInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, LZ4"},
					},
					Engine: MergeTree{
						PartitionBy: "toDate(timestamp_ms / 1000)",
						OrderBy:     "(metric_name, fingerprint, timestamp_ms)",
						TTL:         "toDateTime(timestamp_ms / 1000) + toIntervalSecond(2592000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v2",
				},
			},
		},
		{
			MigrationID: 2,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v2",
					Columns: []Column{
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "timestamp_ms", Type: ColumnTypeInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, LZ4"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v2",
						ShardingKey: "cityHash64(metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v2",
				},
			},
		},
		{
			MigrationID: 3,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v2",
					Columns: []Column{
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "timestamp_ms", Type: ColumnTypeInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'", Codec: "ZSTD(5)"},
						{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					},
					Indexes: []Index{
						{Name: "temporality_index", Expression: "temporality", Type: "SET(3)", Granularity: 1},
					},
					Engine: ReplacingMergeTree{
						MergeTree{
							PartitionBy: "toDate(timestamp_ms / 1000)",
							OrderBy:     "(metric_name, fingerprint)",
							Settings: TableSettings{
								{Name: "index_granularity", Value: "8192"},
							},
						},
					},
				},
			},
			DownItems: []Operation{},
		},
		{
			MigrationID: 4,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v2",
					Columns: []Column{
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "timestamp_ms", Type: ColumnTypeInt64, Codec: "DoubleDelta, LZ4"},
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'", Codec: "ZSTD(5)"},
						{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "''", Codec: "ZSTD(1)"},
						{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false", Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "time_series_v2",
						ShardingKey: "cityHash64(metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{},
		},
		{
			MigrationID: 5,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "exp_hist",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "count", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "sum", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "min", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "max", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "sketch", Type: AggregateFunction{FunctionName: "quantilesDD(0.01, 0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "exp_hist",
				},
			},
		},
		{
			MigrationID: 6,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_exp_hist",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "count", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "sum", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "min", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "max", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
						{Name: "sketch", Type: AggregateFunction{FunctionName: "quantilesDD(0.01, 0.5, 0.75, 0.9, 0.95, 0.99)", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "exp_hist",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_exp_hist",
				},
			},
		},
		{
			MigrationID: 7,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4",
				},
			},
		},
		{
			MigrationID: 8,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "DoubleDelta, ZSTD(1)"},
						{Name: "value", Type: ColumnTypeFloat64, Codec: "Gorilla, ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v4",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4",
				},
			},
		},
		{
			MigrationID: 9,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4",
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
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4",
				},
			},
		},
		{
			MigrationID: 10,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4",
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
						Database:    "signoz_metrics",
						Table:       "time_series_v4",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4",
				},
			},
		},
		{
			MigrationID: 11,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_6hrs",
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
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_6hrs",
				},
			},
		},
		{
			MigrationID: 12,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_6hrs",
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
						Database:    "signoz_metrics",
						Table:       "time_series_v4_6hrs",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_6hrs",
				},
			},
		},
		{
			MigrationID: 13,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1day",
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
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1day",
				},
			},
		},
		{
			MigrationID: 14,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1day",
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
						Database:    "signoz_metrics",
						Table:       "time_series_v4_1day",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1day",
				},
			},
		},
		{
			MigrationID: 15,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week",
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
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week",
				},
			},
		},
		{
			MigrationID: 16,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1week",
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
						Database:    "signoz_metrics",
						Table:       "time_series_v4_1week",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1week",
				},
			},
		},
		{
			MigrationID: 17,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "time_series_v4_6hrs_mv",
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
    labels
FROM signoz_metrics.time_series_v4`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_6hrs_mv",
				},
			},
		},
		{
			MigrationID: 18,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "time_series_v4_1day_mv",
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
    labels
FROM signoz_metrics.time_series_v4`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1day_mv",
				},
			},
		},
		{
			MigrationID: 19,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "time_series_v4_1week_mv",
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
    labels
FROM signoz_metrics.time_series_v4_1day`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week_mv",
				},
			},
		},
		{
			MigrationID: 20,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: AggregatingMergeTree{
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m",
				},
			},
		},
		{
			MigrationID: 21,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: AggregatingMergeTree{
						MergeTree{
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
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m",
				},
			},
		},
		{
			MigrationID: 22,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "samples_v4_agg_5m_mv",
					DestTable: "samples_v4_agg_5m",
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
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m_mv",
				},
			},
		},
		{
			MigrationID: 23,
			UpItems: []Operation{
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "samples_v4_agg_30m_mv",
					DestTable: "samples_v4_agg_30m",
					Query: `SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 as unix_milli,
    anyLast(last) as last,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics.samples_v4_agg_5m 
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m_mv",
				},
			},
		},
		{
			MigrationID: 24,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_5m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v4_agg_5m",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_5m",
				},
			},
		},
		{
			MigrationID: 25,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_30m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v4_agg_30m",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_30m",
				},
			},
		},
		{
			MigrationID: 26,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
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
					Database: "signoz_metrics",
					Table:    "usage",
				},
			},
		},
		{
			MigrationID: 27,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_usage",
					Columns: []Column{
						{Name: "tenant", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "collector_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "exporter_id", Type: ColumnTypeString, Codec: "ZSTD(1)"},
						{Name: "timestamp", Type: DateTimeColumnType{}, Codec: "ZSTD(1)"},
						{Name: "data", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "usage",
						ShardingKey: "cityHash64(rand())",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_usage",
				},
			},
		},
		{
			MigrationID: 28,
			UpItems: []Operation{
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
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
							TTL:         "toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE",
							Settings: TableSettings{
								{Name: "ttl_only_drop_parts", Value: "1"},
							},
						},
					},
				},
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
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
							TTL:         "toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE",
							Settings: TableSettings{
								{Name: "ttl_only_drop_parts", Value: "1"},
							},
						},
					},
				},
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "samples_v4_agg_5m_mv",
					DestTable: "samples_v4_agg_5m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
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
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
				},
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "samples_v4_agg_30m_mv",
					DestTable: "samples_v4_agg_30m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Query: `SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 as unix_milli,
    anyLast(last) as last,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics.samples_v4_agg_5m 
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;`,
				},
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_5m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v4_agg_5m",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_30m",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "last", Type: SimpleAggregateFunction{FunctionName: "anyLast", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "min", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "max", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "sum", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
						{Name: "count", Type: SimpleAggregateFunction{FunctionName: "sum", Arguments: []ColumnType{ColumnTypeUInt64}}, Codec: "ZSTD(1)"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "samples_v4_agg_30m",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
					},
					Engine: ReplacingMergeTree{
						MergeTree: MergeTree{
							PartitionBy: "toDate(unix_milli / 1000)",
							OrderBy:     "(env, temporality, metric_name, fingerprint, unix_milli)",
							TTL:         "toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE",
							Settings: TableSettings{
								{Name: "ttl_only_drop_parts", Value: "1"},
							},
						},
					},
				},
				CreateMaterializedViewOperation{
					Database:  "signoz_metrics",
					ViewName:  "time_series_v4_1week_mv",
					DestTable: "time_series_v4_1week",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "unit", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "type", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "is_monotonic", Type: ColumnTypeBool, Default: "false"},
						{Name: "fingerprint", Type: ColumnTypeUInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "unix_milli", Type: ColumnTypeInt64, Codec: "Delta(8), ZSTD(1)"},
						{Name: "labels", Type: ColumnTypeString, Codec: "ZSTD(5)"},
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
    floor(unix_milli/604800000)*604800000 AS unix_milli,
    labels
FROM signoz_metrics.time_series_v4_1day;`,
				},
				CreateTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1week",
					Columns: []Column{
						{Name: "env", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
						{Name: "temporality", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'Unspecified'"},
						{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
						{Name: "description", Type: LowCardinalityColumnType{ColumnTypeString}, Default: "'default'"},
					},
					Engine: Distributed{
						Database:    "signoz_metrics",
						Table:       "time_series_v4_1week",
						ShardingKey: "cityHash64(env, temporality, metric_name, fingerprint)",
					},
				},
			},
			DownItems: []Operation{
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_5m_mv",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "samples_v4_agg_30m_mv",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_5m",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_samples_v4_agg_30m",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "distributed_time_series_v4_1week",
				},
				DropTableOperation{
					Database: "signoz_metrics",
					Table:    "time_series_v4_1week_mv",
				},
			},
		},
	}
)
