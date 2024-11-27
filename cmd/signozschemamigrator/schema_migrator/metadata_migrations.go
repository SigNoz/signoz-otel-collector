package schemamigrator

var MetadataMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metadata",
				Table:    "attributes_metadata",
				Columns: []Column{
					{Name: "rounded_unix_milli", Type: ColumnTypeUInt64},
					{Name: "data_source", Type: ColumnTypeString},
					{Name: "resource_fingerprint", Type: ColumnTypeUInt64},
					{Name: "fingerprint", Type: ColumnTypeUInt64},
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
						PartitionBy: "toDate(rounded_unix_milli / 1000)",
						OrderBy:     "(data_source, rounded_unix_milli, resource_fingerprint, fingerprint)",
						TTL:         "toDateTime(rounded_unix_milli) + INTERVAL 1296000 SECOND + INTERVAL 1800 SECOND DELETE",
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
					{Name: "rounded_unix_milli", Type: ColumnTypeUInt64},
					{Name: "data_source", Type: ColumnTypeString},
					{Name: "resource_fingerprint", Type: ColumnTypeUInt64},
					{Name: "fingerprint", Type: ColumnTypeUInt64},
					{Name: "resource_attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
					{Name: "attributes", Type: MapColumnType{KeyType: LowCardinalityColumnType{ColumnTypeString}, ValueType: ColumnTypeString}},
				},
				Engine: Distributed{
					Database:    "signoz_metadata",
					Table:       "attributes_metadata",
					ShardingKey: "cityHash64(data_source, resource_fingerprint, fingerprint)",
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
}
