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

func TestUnfoldJSONSubColumnIndexExpr(t *testing.T) {
	testCases := []struct {
		name        string
		expr        string
		wantExpr    string
		wantType    string
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "test-1",
			expr:        "lower(assumeNotNull(dynamicElement(column.path, 'String')))",
			wantExpr:    "column.path",
			wantType:    "String",
			wantError:   false,
			errorSubstr: "",
		},
		{
			name:        "test-2",
			expr:        "lower(assumeNotNull(dynamicElement(column.`path`, 'Int64')))",
			wantExpr:    "column.`path`",
			wantType:    "Int64",
			wantError:   false,
			errorSubstr: "",
		},
		{
			name:        "test-3",
			expr:        "dynamicElement(body.nested.path,'Float64')",
			wantExpr:    "body.nested.path",
			wantType:    "Float64",
			wantError:   true,
			errorSubstr: "invalid expression: dynamicElement(body.nested.path,'Float64')",
		}, {
			name:        "test-4",
			expr:        "lower(assumeNotNull(dynamicElement(body.nested.`order-id`,'Int64')))",
			wantExpr:    "body.nested.`order-id`",
			wantType:    "Int64",
			wantError:   false,
			errorSubstr: "",
		},
		{
			name:        "invalid-expression-empty",
			expr:        "",
			wantExpr:    "",
			wantType:    "",
			wantError:   true,
			errorSubstr: "invalid expression: ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotExpr, gotType, err := UnfoldJSONSubColumnIndexExpr(tc.expr)

			if tc.wantError {
				require.Error(t, err)
				if tc.errorSubstr != "" {
					require.Contains(t, err.Error(), tc.errorSubstr)
				}
				require.Empty(t, gotExpr)
				require.Empty(t, gotType)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantExpr, gotExpr)
				require.Equal(t, tc.wantType, gotType)
			}
		})
	}
}
