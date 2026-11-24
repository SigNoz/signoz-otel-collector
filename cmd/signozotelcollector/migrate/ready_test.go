package migrate

import (
	"context"
	"errors"
	"testing"

	mockhouse "github.com/SigNoz/clickhouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCheckClickhouse(t *testing.T) {
	testCases := []struct {
		name          string
		hosts         [][]any
		queryErr      error
		wantErr       bool
		wantRetryable bool
	}{
		{
			name:  "AllReplicasAnswer",
			hosts: [][]any{{"replica-1"}, {"replica-2"}},
		},
		{
			name:          "ReplicaUnreachable_Retryable",
			queryErr:      errors.New("code: 279, message: All connection tries failed"),
			wantErr:       true,
			wantRetryable: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conn, err := mockhouse.NewClickHouseNative(nil)
			require.NoError(t, err)

			query := conn.ExpectQuery("SELECT hostName() FROM clusterAllReplicas(?, system.one) SETTINGS skip_unavailable_shards = 0").WithArgs("cluster")
			if testCase.queryErr != nil {
				query.WillReturnError(testCase.queryErr)
			} else {
				query.WillReturnRows(mockhouse.NewRows([]mockhouse.ColumnType{{Name: "hostName()", Type: "String"}}, testCase.hosts))
			}

			r := &ready{conn: conn, cluster: "cluster", logger: zap.NewNop()}
			err = r.CheckClickhouse(context.Background())

			if testCase.wantErr {
				require.Error(t, err)
				assert.Equal(t, testCase.wantRetryable, Unwrapb(err).IsRetryable())
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, conn.ExpectationsWereMet())
		})
	}
}
