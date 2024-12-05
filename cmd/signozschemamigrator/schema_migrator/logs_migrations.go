package schemamigrator

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
}
