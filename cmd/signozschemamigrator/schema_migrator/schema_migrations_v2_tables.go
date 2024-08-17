package schemamigrator

var V2Tables = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeInt32},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy:    "id",
						PrimaryKey: "id",
					},
				},
			},
		},
	},
	{
		MigrationID: 2,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeInt32},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy:    "id",
						PrimaryKey: "id",
					},
				},
			},
		},
	},
	{
		MigrationID: 3,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeInt32},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy:    "id",
						PrimaryKey: "id",
					},
				},
			},
		},
	},
}
