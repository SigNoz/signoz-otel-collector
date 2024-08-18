package schemamigrator

import "time"

type MigrationSchemaMigrationRecord struct {
	MigrationID uint64    `ch:"migration_id"`
	Name        string    `ch:"name"`
	Status      string    `ch:"status"`
	Error       string    `ch:"error"`
	CreatedAt   time.Time `ch:"created_at"`
	UpdatedAt   time.Time `ch:"updated_at"`
}

var V2Tables = []SchemaMigrationRecord{
	{
		MigrationID: 1,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_logs",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
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
				Database: "signoz_logs",
				Table:    "distributed_schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: Distributed{
					Database:    "signoz_logs",
					Table:       "schema_migrations_v2",
					ShardingKey: "rand()",
				},
			},
		},
	},
	{
		MigrationID: 3,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
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
		MigrationID: 4,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_traces",
				Table:    "distributed_schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: Distributed{
					Database:    "signoz_traces",
					Table:       "schema_migrations_v2",
					ShardingKey: "rand()",
				},
			},
		},
	},
	{
		MigrationID: 5,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
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
		MigrationID: 6,
		UpItems: []Operation{
			CreateTableOperation{
				Database: "signoz_metrics",
				Table:    "distributed_schema_migrations_v2",
				Columns: []Column{
					{Name: "id", Type: ColumnTypeUInt64},
					{Name: "name", Type: ColumnTypeString},
					{Name: "status", Type: ColumnTypeString},
					{Name: "error", Type: ColumnTypeString},
					{Name: "created_at", Type: DateTime64ColumnType{Precision: 9}},
					{Name: "updated_at", Type: DateTime64ColumnType{Precision: 9}},
				},
				Engine: Distributed{
					Database:    "signoz_metrics",
					Table:       "schema_migrations_v2",
					ShardingKey: "rand()",
				},
			},
		},
	},
}
