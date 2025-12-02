package schemamigrator

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/stretchr/testify/require"
)

func TestAlterTableAddColumn(t *testing.T) {

	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "col-with-name-and-type-and-cluster",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: ColumnTypeString,
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col String",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-codec",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:  "col",
					Type:  ColumnTypeString,
					Codec: "ZSTD(5)",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col String CODEC(ZSTD(5))",
		},
		{
			name: "col-with-name-and-type-without-cluster",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: ColumnTypeString,
				},
			},
			want: "ALTER TABLE db.table ADD COLUMN IF NOT EXISTS col String",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeString,
					Default: "'default'",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col String DEFAULT 'default'",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-int-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeInt64,
					Default: "1",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Int64 DEFAULT 1",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-float-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeFloat64,
					Default: "1.1",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Float64 DEFAULT 1.1",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-bool-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeBool,
					Default: "true",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Bool DEFAULT true",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-date-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeDate,
					Default: "'2021-01-01'",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Date DEFAULT '2021-01-01'",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-uuid-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeUUID,
					Default: "uuid_nil()", // no such function in clickhouse but shows any expression can be passed as default value
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col UUID DEFAULT uuid_nil()",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-default-ip-value",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ColumnTypeIPv4,
					Default: "'127.0.0.1'",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col IPv4 DEFAULT '127.0.0.1'",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-array-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: ArrayColumnType{ColumnTypeInt32},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Array(Int32)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-array-type-with-default",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ArrayColumnType{ColumnTypeInt32},
					Default: "[1, 2, 3]",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Array(Int32) DEFAULT [1, 2, 3]",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-array-type-with-default-and-array-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    ArrayColumnType{ArrayColumnType{ColumnTypeInt32}},
					Default: "[[1, 2, 3], [4, 5, 6]]",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Array(Array(Int32)) DEFAULT [[1, 2, 3], [4, 5, 6]]",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-map-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: MapColumnType{ColumnTypeInt32, ColumnTypeString},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Map(Int32, String)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-map-type-with-default",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    MapColumnType{ColumnTypeInt32, ColumnTypeString},
					Default: "{1: 'a', 2: 'b'}",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Map(Int32, String) DEFAULT {1: 'a', 2: 'b'}",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-tuple-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: TupleColumnType{[]ColumnType{ColumnTypeBool, ColumnTypeString}},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Tuple(Bool, String)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-low-cardinality-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: LowCardinalityColumnType{ColumnTypeString},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col LowCardinality(String)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-low-cardinality-type-with-default",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    LowCardinalityColumnType{ColumnTypeString},
					Default: "'default'",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col LowCardinality(String) DEFAULT 'default'",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-json-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: JSONColumnType{},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col JSON()",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-json-type-with-params",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: JSONColumnType{MaxDynamicPaths: utils.ToPointer(uint(10)), MaxDynamicTypes: utils.ToPointer(uint(2))},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col JSON(max_dynamic_paths=10, max_dynamic_types=2)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-nullable-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: NullableColumnType{ColumnTypeString},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Nullable(String)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-nullable-type-with-default",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    NullableColumnType{ColumnTypeString},
					Default: "'default'",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col Nullable(String) DEFAULT 'default'",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-simple-aggregate-function-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: SimpleAggregateFunction{"Sum", []ColumnType{ColumnTypeInt32}},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col SimpleAggregateFunction(Sum, Int32)",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-simple-aggregate-function-type-with-default",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:    "col",
					Type:    SimpleAggregateFunction{"Sum", []ColumnType{ColumnTypeInt32}},
					Default: "1",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col SimpleAggregateFunction(Sum, Int32) DEFAULT 1",
		},
		{
			name: "col-with-name-and-type-and-cluster-and-aggregate-function-type",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name: "col",
					Type: AggregateFunction{"Sum", []ColumnType{ColumnTypeInt32}},
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col AggregateFunction(Sum, Int32)",
		},
		{
			name: "col-with-alias",
			op: AlterTableAddColumn{
				Database: "db",
				Table:    "table",
				Column: Column{
					Name:  "col",
					Type:  ColumnTypeString,
					Alias: "maincol",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD COLUMN IF NOT EXISTS col String ALIAS maincol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableDropColumn(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "drop-column",
			op:   AlterTableDropColumn{Database: "db", Table: "table", Column: Column{Name: "col"}},
			want: "ALTER TABLE db.table DROP COLUMN IF EXISTS col",
		},
		{
			name: "drop-column-with-cluster",
			op:   AlterTableDropColumn{Database: "db", Table: "table", Column: Column{Name: "col"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster DROP COLUMN IF EXISTS col",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableModifyColumn(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "modify-column-type",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Type: ColumnTypeIPv4}},
			want: "ALTER TABLE db.table MODIFY COLUMN IF EXISTS col IPv4",
		},
		{
			name: "modify-column-type-with-cluster",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Type: ColumnTypeIPv4}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col IPv4",
		},
		{
			name: "modify-column-type-with-default",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Type: ColumnTypeIPv4, Default: "'127.0.0.1'"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col IPv4 DEFAULT '127.0.0.1'",
		},
		{
			name: "modify-column-default",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Default: "'127.0.0.1'"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col DEFAULT '127.0.0.1'",
		},
		{
			name: "modify-column-compression-codec",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Codec: "ZSTD"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col CODEC(ZSTD)",
		},
		{
			name: "modify-column-ttl",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", TTL: "d + INTERVAL 1 DAY"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col TTL d + INTERVAL 1 DAY",
		},
		{
			name: "modify-column-settings",
			op:   AlterTableModifyColumn{Database: "db", Table: "table", Column: Column{Name: "col", Settings: ColumnSettings{ColumnSetting{Name: "min_compress_block_size", Value: "16777216"}}}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col SETTINGS min_compress_block_size = 16777216",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableModifyColumnRemove(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "remove-column-property-ttl",
			op:   AlterTableModifyColumnRemove{Database: "db", Table: "table", Column: Column{Name: "col"}, Property: ColumnPropertyTTL}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col REMOVE TTL",
		},
		{
			name: "remove-column-property-codec",
			op:   AlterTableModifyColumnRemove{Database: "db", Table: "table", Column: Column{Name: "col"}, Property: ColumnPropertyCodec}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col REMOVE CODEC",
		},
		{
			name: "remove-column-property-default",
			op:   AlterTableModifyColumnRemove{Database: "db", Table: "table", Column: Column{Name: "col"}, Property: ColumnPropertyDefault}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col REMOVE DEFAULT",
		},
		{
			name: "remove-column-property-settings",
			op:   AlterTableModifyColumnRemove{Database: "db", Table: "table", Column: Column{Name: "col"}, Property: ColumnPropertySettings}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col REMOVE SETTINGS",
		},
		{
			name: "remove-column-property-materialized",
			op:   AlterTableModifyColumnRemove{Database: "db", Table: "table", Column: Column{Name: "col"}, Property: ColumnPropertyMaterialized}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MODIFY COLUMN IF EXISTS col REMOVE MATERIALIZED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableMaterializeColumn(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "materialize-column",
			op:   AlterTableMaterializeColumn{Database: "db", Table: "table", Column: Column{Name: "col"}},
			want: "ALTER TABLE db.table MATERIALIZE COLUMN col",
		},
		{
			name: "materialize-column-with-cluster",
			op:   AlterTableMaterializeColumn{Database: "db", Table: "table", Column: Column{Name: "col"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MATERIALIZE COLUMN col",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
