package schemamigrator

import (
	"fmt"
	"time"
)

var MetadataMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "attributes_metadata",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeUInt64},
					{Name: "data_source", Type: ColumnTypeString},
					{Name: "resource_fingerprint", Type: ColumnTypeUInt64},
					{Name: "attrs_fingerprint", Type: ColumnTypeUInt64},
					{Name: "resource_attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
					{Name: "attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
				},
				Indexes: []Index{
					{Name: "idx_resource_attributes_map_keys", Expression: "mapKeys(resource_attributes)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "idx_attributes_map_keys", Expression: "mapKeys(attributes)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
					{Name: "idx_resource_attributes_map_values", Expression: "mapValues(resource_attributes)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
					{Name: "idx_attributes_map_values", Expression: "mapValues(attributes)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toDate(unix_milli / 1000)",
						OrderBy:     "(data_source, unix_milli, resource_fingerprint, attrs_fingerprint)",
						TTL:         "toDateTime(unix_milli / 1000) + INTERVAL 2592000 SECOND DELETE",
						Settings: TableSettings{
							{Name: "ttl_only_drop_parts", Value: "1"},
							{Name: "index_granularity", Value: "8192"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_attributes_metadata",
				Columns: []Column{
					{Name: "unix_milli", Type: ColumnTypeUInt64},
					{Name: "data_source", Type: ColumnTypeString},
					{Name: "resource_fingerprint", Type: ColumnTypeUInt64},
					{Name: "attrs_fingerprint", Type: ColumnTypeUInt64},
					{Name: "resource_attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
					{Name: "attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
				},
				Engine: Distributed{
					Database:    "signoz_metadata",
					Table:       "attributes_metadata",
					ShardingKey: "cityHash64(data_source, resource_fingerprint, attrs_fingerprint)",
				},
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_attributes_metadata",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "attributes_metadata",
			},
		},
	},
	{
		MigrationID: 1001,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "column_evolution_metadata",
				Columns: []Column{
					{Name: "signal", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "column_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "column_type", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "version", Type: ColumnTypeUInt32, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "release_time", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
				},
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						OrderBy:     "(signal, column_name, column_type, field_context, field_name, version)",
						PartitionBy: "toDate(release_time / 1000000000)",
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_column_evolution_metadata",
				Columns: []Column{
					{Name: "signal", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "column_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "column_type", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_context", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "field_name", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "version", Type: ColumnTypeUInt32, Codec: "DoubleDelta, ZSTD(1)"},
					{Name: "release_time", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeFloat64}}, Codec: "ZSTD(1)"},
				},
				Engine: Distributed{
					Database:    "signoz_metadata",
					Table:       "column_evolution_metadata",
					ShardingKey: "cityHash64(signal,column_name)",
				},
			},
			InsertIntoTable{
				Database:    "signoz_metadata",
				Table:       "distributed_column_evolution_metadata",
				LightWeight: true,
				Synchronous: true,
				Columns:     []string{"signal", "column_name", "column_type", "field_context", "field_name", "version", "release_time"},
				Values:      "('logs', 'resources_string', 'Map(LowCardinality(String), Float64)', 'resource', '__all__', 0, 0)",
			},
			InsertIntoTable{
				Database:    "signoz_metadata",
				Table:       "distributed_column_evolution_metadata",
				LightWeight: true,
				Synchronous: true,
				Columns:     []string{"signal", "column_name", "column_type", "field_context", "field_name", "version", "release_time"},
				Values:      fmt.Sprintf("('logs', 'resource', 'JSON()', 'resource', '__all__', 1, %d)", time.Now().UnixNano()),
			},
			InsertIntoTable{
				Database:    "signoz_metadata",
				Table:       "distributed_column_evolution_metadata",
				LightWeight: true,
				Synchronous: true,
				Columns:     []string{"signal", "column_name", "column_type", "field_context", "field_name", "version", "release_time"},
				Values:      "('traces', 'resources_string', 'Map(LowCardinality(String), Float64)', 'resource', '__all__', 0, 0)",
			},
			InsertIntoTable{
				Database:    "signoz_metadata",
				Table:       "distributed_column_evolution_metadata",
				LightWeight: true,
				Synchronous: true,
				Columns:     []string{"signal", "column_name", "column_type", "field_context", "field_name", "version", "release_time"},
				Values:      fmt.Sprintf("('traces', 'resource', 'JSON()', 'resource', '__all__', 1, %d)", time.Now().UnixNano()),
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_column_evolution_metadata",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "column_evolution_metadata",
			},
		},
	},
	{
		MigrationID: 1002,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "field_values_sets",
				Columns:  fieldValuesSetsColumns,
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						OrderBy: "(signal, source, metric_name, field_name, field_context, field_data_type, string_value, number_value, resource_hash, attrs_hash)",
						TTL:     "toDateTime(intDiv(last_seen, 1000)) + toIntervalDay(30)",
						Settings: TableSettings{
							{Name: "allow_nullable_key", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_field_values_sets",
				Columns:  fieldValuesSetsColumns,
				Engine: Distributed{
					Database:    "signoz_metadata",
					Table:       "field_values_sets",
					ShardingKey: "cityHash64(signal, resource_hash)",
				},
			},
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "field_values_daily",
				Columns:  fieldValuesDailyColumns,
				Engine: AggregatingMergeTree{
					MergeTree: MergeTree{
						PartitionBy: "toMonday(day)",
						OrderBy:     "(signal, source, metric_name, field_name, field_context, field_data_type, string_value, number_value, day)",
						TTL:         "day + toIntervalDay(35)",
						Settings: TableSettings{
							{Name: "allow_nullable_key", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_field_values_daily",
				Columns:  fieldValuesDailyColumns,
				Engine: Distributed{
					Database:    "signoz_metadata",
					Table:       "field_values_daily",
					ShardingKey: "cityHash64(signal, field_name)",
				},
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_metadata",
				ViewName:  "field_values_daily_mv",
				DestTable: "field_values_daily",
				Query: `SELECT
    signal,
    source,
    scope_metric AS metric_name,
    field_context,
    field_name,
    field_data_type,
    string_value,
    number_value,
    toDate(toDateTime(intDiv(first_seen, 1000), 'UTC')) AS day,
    uniqState(cityHash64(resource_hash, attrs_hash)) AS holders
FROM signoz_metadata.field_values_sets
ARRAY JOIN if(field_values_sets.metric_name = '', [''], [field_values_sets.metric_name, '']) AS scope_metric
GROUP BY signal, source, scope_metric, field_context, field_name, field_data_type, string_value, number_value, day`,
			},
		},
		DownItems: []Operation{
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "field_values_daily_mv",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_field_values_daily",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "field_values_daily",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "distributed_field_values_sets",
			},
			DropTableOperation{
				Database: "signoz_metadata",
				Table:    "field_values_sets",
			},
		},
	},
}

var fieldContextColumnType = EnumerationColumnType{
	Values: []string{"'resource' = 1", "'attribute' = 2", "'scope' = 3", "'span' = 4", "'log' = 5", "'event' = 6", "'body' = 7"},
	Size:   8,
}

var fieldDataTypeColumnType = EnumerationColumnType{
	Values: []string{"'string' = 1", "'number' = 2", "'bool' = 3"},
	Size:   8,
}

var fieldValuesSetsColumns = []Column{
	{Name: "signal", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "source", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "field_context", Type: fieldContextColumnType},
	{Name: "field_name", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "field_data_type", Type: fieldDataTypeColumnType},
	{Name: "string_value", Type: ColumnTypeString},
	{Name: "number_value", Type: NullableColumnType{ColumnTypeFloat64}},
	{Name: "resource_hash", Type: ColumnTypeUInt64},
	{Name: "attrs_hash", Type: ColumnTypeUInt64},
	{Name: "in_hash", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeBool}}},
	{Name: "first_seen", Type: SimpleAggregateFunction{FunctionName: "min", Arguments: []ColumnType{ColumnTypeUInt64}}},
	{Name: "last_seen", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{ColumnTypeUInt64}}},
	{Name: "inserted_at", Type: SimpleAggregateFunction{FunctionName: "max", Arguments: []ColumnType{DateTime64ColumnType{Precision: 3}}}},
}

var fieldValuesDailyColumns = []Column{
	{Name: "signal", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "source", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "metric_name", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "field_context", Type: fieldContextColumnType},
	{Name: "field_name", Type: LowCardinalityColumnType{ColumnTypeString}},
	{Name: "field_data_type", Type: fieldDataTypeColumnType},
	{Name: "string_value", Type: ColumnTypeString},
	{Name: "number_value", Type: NullableColumnType{ColumnTypeFloat64}},
	{Name: "day", Type: ColumnTypeDate},
	{Name: "holders", Type: AggregateFunction{FunctionName: "uniq", Arguments: []ColumnType{ColumnTypeUInt64}}},
}
