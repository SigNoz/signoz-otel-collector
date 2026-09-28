package schemamigrator

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializedKeyIndexesToSQL(t *testing.T) {
	op := MaterializedKeyIndexes{Database: "signoz_traces", Table: "signoz_index_v3", MapColumn: "attributes_string", JSONColumn: "attributes", IndexType: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1}
	want := `SELECT DISTINCT extract(default_expression, '^attributes_string\\[\'(.+)\'\\]$') AS key FROM system.columns WHERE database = 'signoz_traces' AND table = 'signoz_index_v3' AND default_kind IN ('DEFAULT', 'MATERIALIZED') AND key != '' ORDER BY key`
	assert.Equal(t, want, op.ToSQL())
}

func TestMaterializedKeyIndexesResolve(t *testing.T) {
	testCases := []struct {
		name string
		drop bool
		want []string
	}{
		{
			name: "AddIndexes",
			want: []string{
				"ALTER TABLE signoz_traces.signoz_index_v3 ON CLUSTER cluster ADD INDEX IF NOT EXISTS `attributes.gen_ai.request.model_String_ngrambf_v1` attributes.`gen_ai.request.model`::String TYPE ngrambf_v1(4, 5000, 2, 0) GRANULARITY 1",
				"ALTER TABLE signoz_traces.signoz_index_v3 ON CLUSTER cluster ADD INDEX IF NOT EXISTS `attributes.odd\\`key_String_ngrambf_v1` attributes.`odd\\`key`::String TYPE ngrambf_v1(4, 5000, 2, 0) GRANULARITY 1",
			},
		},
		{
			name: "DropIndexes",
			drop: true,
			want: []string{
				"ALTER TABLE signoz_traces.signoz_index_v3 ON CLUSTER cluster DROP INDEX IF EXISTS `attributes.gen_ai.request.model_String_ngrambf_v1`",
				"ALTER TABLE signoz_traces.signoz_index_v3 ON CLUSTER cluster DROP INDEX IF EXISTS `attributes.odd\\`key_String_ngrambf_v1`",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mock, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
			require.NoError(t, err)

			rows := cmock.NewRows([]cmock.ColumnType{{Name: "key", Type: "String"}}, [][]any{{"gen_ai.request.model"}, {"odd`key"}})
			mock.ExpectQuery("FROM system.columns").WillReturnRows(rows)

			op := MaterializedKeyIndexes{Database: "signoz_traces", Table: "signoz_index_v3", MapColumn: "attributes_string", JSONColumn: "attributes", IndexType: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1, Drop: testCase.drop}
			ops, err := op.Resolve(context.Background(), mock)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			got := make([]string, 0, len(ops))
			for _, resolved := range ops {
				got = append(got, resolved.OnCluster("cluster").ToSQL())
			}
			assert.Equal(t, testCase.want, got)
		})
	}
}
