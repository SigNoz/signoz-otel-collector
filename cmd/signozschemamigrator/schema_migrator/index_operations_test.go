package schemamigrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlterTableAddIndex(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "add-index",
			op:   AlterTableAddIndex{Database: "db", Table: "table", Index: Index{Name: "idx", Expression: "mapKeys(numberTagMap)", Type: "bloom_filter", Granularity: 1}},
			want: "ALTER TABLE db.table ADD INDEX IF NOT EXISTS idx mapKeys(numberTagMap) TYPE bloom_filter GRANULARITY 1",
		},
		{
			name: "add-index-on-cluster",
			op:   AlterTableAddIndex{Database: "db", Table: "table", Index: Index{Name: "idx", Expression: "mapKeys(numberTagMap)", Type: "bloom_filter", Granularity: 1}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD INDEX IF NOT EXISTS idx mapKeys(numberTagMap) TYPE bloom_filter GRANULARITY 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableDropIndex(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "drop-index",
			op:   AlterTableDropIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}},
			want: "ALTER TABLE db.table DROP INDEX IF EXISTS idx",
		},
		{
			name: "drop-index-on-cluster",
			op:   AlterTableDropIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster DROP INDEX IF EXISTS idx",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableMaterializeIndex(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "materialize-index",
			op:   AlterTableMaterializeIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}},
			want: "ALTER TABLE db.table MATERIALIZE INDEX IF EXISTS idx",
		},
		{
			name: "materialize-index-on-cluster",
			op:   AlterTableMaterializeIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MATERIALIZE INDEX IF EXISTS idx",
		},
		{
			name: "materialize-index-in-partition",
			op:   AlterTableMaterializeIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}, Partition: "partition"},
			want: "ALTER TABLE db.table MATERIALIZE INDEX IF EXISTS idx IN PARTITION partition",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableClearIndex(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "clear-index",
			op:   AlterTableClearIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}},
			want: "ALTER TABLE db.table CLEAR INDEX IF EXISTS idx",
		},
		{
			name: "clear-index-on-cluster",
			op:   AlterTableClearIndex{Database: "db", Table: "table", Index: Index{Name: "idx"}}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster CLEAR INDEX IF EXISTS idx",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
