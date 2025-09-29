package schemamigrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTable(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "create-table-without-any-engine-params",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
				},
				Engine: MergeTree{},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16) ENGINE = MergeTree",
		},
		{
			name: "create-table-without-any-engine-params-with-alias",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
					{
						Name:  "id_alias",
						Type:  ColumnTypeInt16,
						Alias: "id",
					},
				},
				Engine: MergeTree{},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16, id_alias Int16 ALIAS id) ENGINE = MergeTree",
		},
		{
			name: "create-table-with-replacing-merge-tree-engine-params",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy: "id",
					},
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16) ENGINE = ReplacingMergeTree ORDER BY id",
		},
		{
			name: "create-table-with-engine-param-order-by",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
				},
				Engine: MergeTree{
					OrderBy: "id",
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16) ENGINE = MergeTree ORDER BY id",
		},
		{
			name: "create-table-with-engine-param-order-by-and-partition-by",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
					{
						Name: "ts",
						Type: DateTime64ColumnType{
							Precision: 3,
						},
					},
				},
				Engine: MergeTree{
					OrderBy:     "id",
					PartitionBy: "toYYYYMM(ts)",
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16, ts DateTime64(3)) ENGINE = MergeTree ORDER BY id PARTITION BY toYYYYMM(ts)",
		},
		{
			name: "create-table-with-engine-param-order-by-and-partition-by-and-ttl",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
					{
						Name: "ts",
						Type: DateTime64ColumnType{
							Precision: 3,
						},
					},
				},
				Engine: MergeTree{
					OrderBy:     "id",
					PartitionBy: "toYYYYMM(ts)",
					TTL:         "ts + INTERVAL 1 DAY",
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16, ts DateTime64(3)) ENGINE = MergeTree ORDER BY id PARTITION BY toYYYYMM(ts) TTL ts + INTERVAL 1 DAY",
		},
		{
			name: "create-table-with-engine-param-order-by-and-partition-by-and-ttl-and-primary-key",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
					{
						Name: "ts",
						Type: DateTime64ColumnType{
							Precision: 3,
						},
					},
					{
						Name: "value",
						Type: ColumnTypeFloat64,
					},
				},
				Engine: MergeTree{
					OrderBy:     "id",
					PartitionBy: "toYYYYMM(ts)",
					TTL:         "ts + INTERVAL 1 DAY",
					PrimaryKey:  "(id, ts)",
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16, ts DateTime64(3), value Float64) ENGINE = MergeTree PRIMARY KEY (id, ts) ORDER BY id PARTITION BY toYYYYMM(ts) TTL ts + INTERVAL 1 DAY",
		},
		{
			name: "create-table-with-engine-param-order-by-and-partition-by-and-ttl-and-primary-key-and-ttl-and-sample-by",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
					{
						Name: "ts",
						Type: DateTime64ColumnType{
							Precision: 3,
						},
					},
				},
				Engine: MergeTree{
					OrderBy:     "id",
					PartitionBy: "toYYYYMM(ts)",
					TTL:         "ts + INTERVAL 1 DAY",
					PrimaryKey:  "(id, ts)",
					SampleBy:    "id",
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table (id Int16, ts DateTime64(3)) ENGINE = MergeTree PRIMARY KEY (id, ts) ORDER BY id PARTITION BY toYYYYMM(ts) SAMPLE BY id TTL ts + INTERVAL 1 DAY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestDropTable(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "drop-table",
			op: DropTableOperation{
				Database: "db",
				Table:    "table",
			},
			want: "DROP TABLE IF EXISTS db.table",
		},
		{
			name: "drop-table-with-settings",
			op: DropTableOperation{
				Database: "db",
				Table:    "table",
				Settings: TableSettings{{Name: "max_table_size_to_drop", Value: "0"}},
			},
			want: "DROP TABLE IF EXISTS db.table SETTINGS max_table_size_to_drop = 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestCreateMaterializedView(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "create-materialized-view",
			op: CreateMaterializedViewOperation{
				Database:  "db",
				ViewName:  "view",
				DestTable: "dest_table",
				Query:     "SELECT id, ts, value FROM db.table",
			},
			want: "CREATE MATERIALIZED VIEW IF NOT EXISTS db.view TO db.dest_table AS SELECT id, ts, value FROM db.table",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestModifyQueryMaterializedView(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "modify-materialized-view",
			op: ModifyQueryMaterializedViewOperation{
				Database: "db",
				ViewName: "view",
				Query:    "SELECT id, ts, value FROM db.table",
			},
			want: "ALTER TABLE db.view MODIFY QUERY SELECT id, ts, value FROM db.table",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestCreateWithCluster(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "create-table-with-replacing-merge-tree-engine-params",
			op: CreateTableOperation{
				Database: "db",
				Table:    "table",
				Columns: []Column{
					{
						Name: "id",
						Type: ColumnTypeInt16,
					},
				},
				Engine: ReplacingMergeTree{
					MergeTree{
						OrderBy: "id",
					},
				},
			},
			want: "CREATE TABLE IF NOT EXISTS db.table ON CLUSTER cluster (id Int16) ENGINE = ReplacingMergeTree ORDER BY id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op := tc.op.OnCluster("cluster")
			require.Equal(t, tc.want, op.ToSQL())
		})
	}
}

func TestTruncateTable(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "truncate-table",
			op: TruncateTableOperation{
				Database: "db",
				Table:    "table",
			},
			want: "TRUNCATE TABLE IF EXISTS db.table",
		},
		{
			name: "truncate-table-with-cluster",
			op: TruncateTableOperation{
				Database: "db",
				Table:    "table",
				cluster:  "cluster",
			},
			want: "TRUNCATE TABLE IF EXISTS db.table ON CLUSTER cluster",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableModifyTTL(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "alter-table-modify-ttl",
			op: AlterTableModifyTTL{
				Database: "db",
				Table:    "table",
				TTL:      "ts + INTERVAL 1 DAY",
			},
			want: "ALTER TABLE db.table MODIFY TTL ts + INTERVAL 1 DAY SETTINGS materialize_ttl_after_modify = 0",
		},
		{
			name: "alter-table-modify-ttl-with-cluster",
			op: AlterTableModifyTTL{
				Database: "db",
				Table:    "table",
				TTL:      "ts + INTERVAL 1 DAY",
				cluster:  "cluster",
			},
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY TTL ts + INTERVAL 1 DAY SETTINGS materialize_ttl_after_modify = 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableDropTTL(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "alter-table-drop-ttl",
			op: AlterTableDropTTL{
				Database: "db",
				Table:    "table",
			},
			want: "ALTER TABLE db.table REMOVE TTL",
		},
		{
			name: "alter-table-drop-ttl-with-cluster",
			op: AlterTableDropTTL{
				Database: "db",
				Table:    "table",
				cluster:  "cluster",
			},
			want: "ALTER TABLE db.table ON CLUSTER cluster REMOVE TTL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
