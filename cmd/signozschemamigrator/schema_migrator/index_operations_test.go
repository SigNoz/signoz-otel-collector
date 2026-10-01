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
			expr:        "assumeNotNull(dynamicElement(body.nested.`order-id`,'Int64'))",
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

func TestSimpleJSONSubColumnIndexExpr(t *testing.T) {
	testCases := []struct {
		name       string
		column     string
		path       string
		typeColumn string
		want       string
	}{
		{
			name:       "DottedAttributePath_String",
			column:     "attributes",
			path:       "http.route",
			typeColumn: "String",
			want:       "attributes.http.route::String",
		},
		{
			name:       "PromotedColumn_String",
			column:     "attributes_promoted",
			path:       "http.method",
			typeColumn: "String",
			want:       "attributes_promoted.http.method::String",
		},
		{
			name:       "SegmentNeedingBackticks_Backticked",
			column:     "attributes",
			path:       "user-name",
			typeColumn: "String",
			want:       "attributes.`user-name`::String",
		},
		{
			name:       "NumberType",
			column:     "attributes",
			path:       "http.status_code",
			typeColumn: "Int64",
			want:       "attributes.http.status_code::Int64",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, SimpleJSONSubColumnIndexExpr(testCase.column, testCase.path, testCase.typeColumn))
		})
	}
}

func TestSimpleJSONSubColumnIndexName(t *testing.T) {
	testCases := []struct {
		name   string
		column string
		path   string
		want   string
	}{
		{
			name:   "DottedAttributePath",
			column: "attributes",
			path:   "rpc.method",
			want:   "idx_attributes_rpc$$method",
		},
		{
			name:   "AlreadyBacktickedPath_Trimmed",
			column: "attributes",
			path:   "`messaging.operation`",
			want:   "idx_attributes_messaging$$operation",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, SimpleJSONSubColumnIndexName(testCase.column, testCase.path))
		})
	}
}

func TestUnfoldSimpleJSONSubColumnIndexExpr(t *testing.T) {
	testCases := []struct {
		name        string
		expr        string
		wantExpr    string
		wantType    string
		wantError   bool
		errorSubstr string
	}{
		{
			name:     "CastString",
			expr:     "CAST(attributes.http.route, 'String')",
			wantExpr: "attributes.http.route",
			wantType: "String",
		},
		{
			name:     "CastBacktickedSegment",
			expr:     "CAST(attributes.`user-name`, 'String')",
			wantExpr: "attributes.`user-name`",
			wantType: "String",
		},
		{
			name:        "FoldedFormRejected",
			expr:        "lower(assumeNotNull(dynamicElement(column.path, 'String')))",
			wantError:   true,
			errorSubstr: "invalid expression",
		},
		{
			name:        "Empty",
			expr:        "",
			wantError:   true,
			errorSubstr: "invalid expression: ",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotExpr, gotType, err := UnfoldSimpleJSONSubColumnIndexExpr(testCase.expr)
			if testCase.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.errorSubstr)
				require.Empty(t, gotExpr)
				require.Empty(t, gotType)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.wantExpr, gotExpr)
			require.Equal(t, testCase.wantType, gotType)
		})
	}
}
