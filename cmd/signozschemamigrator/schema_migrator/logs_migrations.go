package schemamigrator

import (
	"time"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

var LogsMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_bool_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_float64_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "attribute_keys_string_final_mv",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "resource_keys_string_final_mv",
			},
		},
		DownItems: []Operation{
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_bool_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_bool)) AS name,
'Bool' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_float64_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_number)) AS name,
'Float64' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "attribute_keys_string_final_mv",
				DestTable: "logs_attribute_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(attributes_string)) AS name,
'String' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
			CreateMaterializedViewOperation{
				Database:  "signoz_logs",
				ViewName:  "resource_keys_string_final_mv",
				DestTable: "logs_resource_keys",
				Columns: []Column{
					{Name: "name", Type: ColumnTypeString},
					{Name: "datatype", Type: ColumnTypeString},
				},
				Query: `SELECT DISTINCT
arrayJoin(mapKeys(resources_string)) AS name,
'String' AS datatype
FROM signoz_logs.logs_v2
ORDER BY name ASC`,
			},
		},
	},
	{
		MigrationID: 1001,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
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
				Database: "signoz_logs",
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
					Database:    "signoz_logs",
					Table:       "tag_attributes_v2",
					ShardingKey: "cityHash64(rand())",
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1002,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				Column: Column{
					Name:    "timestamp",
					Type:    DateTimeColumnType{},
					Default: "toDateTime(now())",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				Column: Column{
					Name:    "timestamp",
					Type:    DateTimeColumnType{},
					Default: "toDateTime(now())",
				},
			},
			AlterTableModifyTTL{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				TTL:      "timestamp + INTERVAL 15 DAY",
				Settings: ModifyTTLSettings{
					MaterializeTTLAfterModify: false,
				},
			},
			AlterTableModifyTTL{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				TTL:      "timestamp + INTERVAL 15 DAY",
				Settings: ModifyTTLSettings{
					MaterializeTTLAfterModify: false,
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1003,
		UpItems: []Operation{
			AlterTableMaterializeColumn{
				Database: "signoz_logs",
				Table:    "logs_attribute_keys",
				Column: Column{
					Name: "timestamp",
				},
			},
			AlterTableMaterializeColumn{
				Database: "signoz_logs",
				Table:    "logs_resource_keys",
				Column: Column{
					Name: "timestamp",
				},
			},
		},
		DownItems: []Operation{},
	},
	{
		MigrationID: 1004,
		UpItems: []Operation{
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column: Column{
					Name:  "resource",
					Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](100)},
					Codec: "ZSTD(1)",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
				Column: Column{
					Name:  "resource",
					Type:  JSONColumnType{MaxDynamicPaths: utils.ToPointer[uint](100)},
					Codec: "ZSTD(1)",
				},
			},
		},
		DownItems: []Operation{
			AlterTableDropColumn{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
				Column: Column{
					Name: "resource",
				},
			},
			AlterTableDropColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column: Column{
					Name: "resource",
				},
			},
		},
	},
	{
		MigrationID: 1005,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "path_types",
				Columns: []Column{
					{Name: "path", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "type", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "last_seen", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						OrderBy:     "(path, type)",
						PartitionBy: "toDate(last_seen / 1000000000)",
						TTL:         "toDateTime(last_seen / 1000000000) + toIntervalSecond(1296000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
							{Name: "ttl_only_drop_parts", Value: "1"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_path_types",
				Columns: []Column{
					{Name: "path", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "type", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "last_seen", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "path_types",
					ShardingKey: "cityHash64(path, type)",
				},
			},
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "promoted_paths",
				Columns: []Column{
					{Name: "path", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "created_at", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
				},
				Engine: ReplacingMergeTree{
					MergeTree: MergeTree{
						OrderBy:     "path",
						PartitionBy: "toDate(created_at / 1000000000)",
						Settings: TableSettings{
							{Name: "index_granularity", Value: "8192"},
						},
					},
				},
			},
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_promoted_paths",
				Columns: []Column{
					{Name: "path", Type: ColumnTypeString, Codec: "ZSTD(1)"},
					{Name: "created_at", Type: ColumnTypeUInt64, Codec: "DoubleDelta, LZ4"},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "promoted_paths",
					ShardingKey: "cityHash64(path)",
				},
			},
			AlterTableModifySettings{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Settings: TableSettings{
					{Name: "object_serialization_version", Value: "'v3'"},
					{Name: "object_shared_data_serialization_version", Value: "'advanced'"},
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column: Column{
					Name: "body_v2",
					Type: JSONColumnType{
						MaxDynamicPaths: utils.ToPointer[uint](0),
					},
					Codec: "ZSTD(1)",
				},
				After: &Column{
					Name: "body",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
				Column: Column{
					Name: "body_v2",
					Type: JSONColumnType{
						MaxDynamicPaths: utils.ToPointer[uint](0),
					},
					Codec: "ZSTD(1)",
				},
				After: &Column{
					Name: "body",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column: Column{
					Name:  "promoted",
					Type:  JSONColumnType{},
					Codec: "ZSTD(1)",
				},
				After: &Column{
					Name: "body_v2",
				},
			},
			AlterTableAddColumn{
				Database: "signoz_logs",
				Table:    "distributed_logs_v2",
				Column: Column{
					Name:  "promoted",
					Type:  JSONColumnType{},
					Codec: "ZSTD(1)",
				},
				After: &Column{
					Name: "body_v2",
				},
			},
			InsertIntoTable{
				Database: "signoz_logs",
				Table:    "distributed_promoted_paths",
				Columns:  []string{"path", "created_at"},
				Values: [][]any{
					{"message", time.Now().UnixMilli()},
				},
			},
			AlterTableAddIndex{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Index: Index{
					Name:        JSONSubColumnIndexName("promoted.message", IndexTypeTokenBF),
					Expression:  JSONSubColumnIndexExpr("promoted.message"),
					Type:        "tokenbf_v1(10000, 2, 0)",
					Granularity: 1,
				},
			},
			AlterTableAddIndex{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Index: Index{
					Name:        JSONSubColumnIndexName("promoted.message", IndexTypeNGramBF),
					Expression:  JSONSubColumnIndexExpr("promoted.message"),
					Type:        "ngrambf_v1(4, 60000, 5, 0)",
					Granularity: 1,
				},
			},
		},
		DownItems: []Operation{
			AlterTableDropColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column:   Column{Name: "promoted"},
			},
			AlterTableDropColumn{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Column:   Column{Name: "body_v2"},
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "path_types",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_path_types",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "promoted_paths",
			},
			DropTableOperation{
				Database: "signoz_logs",
				Table:    "distributed_promoted_paths",
			},
			AlterTableDropIndex{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Index: Index{
					Name: JSONSubColumnIndexName("promoted.message", IndexTypeNGramBF),
				},
			},
			AlterTableDropIndex{
				Database: "signoz_logs",
				Table:    "logs_v2",
				Index: Index{
					Name: JSONSubColumnIndexName("promoted.message", IndexTypeTokenBF),
				},
			},
		},
	},
}
