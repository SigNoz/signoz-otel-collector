package schemamigrator

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	mockhouse "github.com/SigNoz/clickhouse-go-mock"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// recordingConn records Select arguments, which the mock does not check.
type recordingConn struct {
	driver.Conn
	selectCalls [][]any
}

func (c *recordingConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	c.selectCalls = append(c.selectCalls, args)
	return c.Conn.Select(ctx, dest, query, args...)
}

type mockManager struct {
	manager  *MigrationManager
	mock     mockhouse.ClickConnMockCommon
	recorder *recordingConn
}

func newMockManager(t *testing.T, cluster string, retry *backoff.ExponentialBackOff, opts ...Option) mockManager {
	t.Helper()

	conn, err := mockhouse.NewClickHouseNative(nil)
	require.NoError(t, err)
	recorder := &recordingConn{Conn: conn}

	if retry == nil {
		retry = backoff.NewExponentialBackOff()
		retry.InitialInterval = time.Millisecond
		retry.MaxElapsedTime = time.Second
	}

	manager, err := NewMigrationManager(append([]Option{
		WithClusterName(cluster),
		WithConn(recorder),
		WithLogger(zap.NewNop()),
		WithBackoff(retry),
	}, opts...)...)
	require.NoError(t, err)

	return mockManager{manager: manager, mock: conn, recorder: recorder}
}

func (m mockManager) expectReplicas(cluster string, names ...string) {
	values := make([][]any, 0, len(names))
	for i, name := range names {
		values = append(values, []any{replicaID(i + 1), name})
	}
	m.mock.ExpectQuery("SELECT DISTINCT toString(serverUUID()) AS id, concat(hostName(), ':', toString(tcpPort())) AS name FROM clusterAllReplicas($1, system.one) ORDER BY name SETTINGS skip_unavailable_shards = 0").
		WithArgs(cluster).
		WillReturnRows(mockhouse.NewRows([]mockhouse.ColumnType{
			{Name: "id", Type: "String"},
			{Name: "name", Type: "String"},
		}, values))
}

func mutationRows(pending int) *mockhouse.Rows {
	values := make([][]any, 0, pending)
	for range pending {
		values = append(values, []any{"signoz_logs", "logs_v2", "MATERIALIZE COLUMN x", "mutation_1.txt", ""})
	}
	return mockhouse.NewRows([]mockhouse.ColumnType{
		{Name: "database", Type: "String"},
		{Name: "table", Type: "String"},
		{Name: "command", Type: "String"},
		{Name: "mutation_id", Type: "String"},
		{Name: "latest_fail_reason", Type: "String"},
	}, values)
}

func TestWaitForRunningMutations(t *testing.T) {
	mutationsQuery := "SELECT database, table, command, mutation_id, latest_fail_reason FROM clusterAllReplicas($1, system.mutations) WHERE toString(serverUUID()) = $2 AND is_done = 0 SETTINGS skip_unavailable_shards = 0"

	testCases := []struct {
		name         string
		retry        func() *backoff.ExponentialBackOff
		replicas     []string
		pendingPolls map[string]int
		queryErr     error
		wantErr      error
		wantCalls    [][]any
	}{
		{
			name:      "NoPendingMutations_SameHostName_PollsEachReplica",
			replicas:  []string{"clickhouse:9000", "clickhouse:9000"},
			wantCalls: [][]any{{"cluster-a", replicaID(1)}, {"cluster-a", replicaID(2)}},
		},
		{
			name:         "PendingOnOneReplica_WaitsForIt",
			replicas:     []string{"host-a:9000", "host-b:9000"},
			pendingPolls: map[string]int{replicaID(1): 2},
			wantCalls:    [][]any{{"cluster-a", replicaID(1)}, {"cluster-a", replicaID(1)}, {"cluster-a", replicaID(1)}, {"cluster-a", replicaID(2)}},
		},
		{
			// One replica's wait needs 400ms of the 550ms budget; both together would need 700ms.
			name: "PendingOnEveryReplica_EachGetsItsOwnBudget",
			retry: func() *backoff.ExponentialBackOff {
				retry := backoff.NewExponentialBackOff()
				retry.InitialInterval = 100 * time.Millisecond
				retry.Multiplier = 1
				retry.RandomizationFactor = 0
				retry.MaxElapsedTime = 550 * time.Millisecond
				return retry
			},
			replicas:     []string{"host-c:9000", "host-d:9000"},
			pendingPolls: map[string]int{replicaID(1): 3, replicaID(2): 3},
			wantCalls: [][]any{
				{"cluster-a", replicaID(1)}, {"cluster-a", replicaID(1)}, {"cluster-a", replicaID(1)}, {"cluster-a", replicaID(1)},
				{"cluster-a", replicaID(2)}, {"cluster-a", replicaID(2)}, {"cluster-a", replicaID(2)}, {"cluster-a", replicaID(2)},
			},
		},
		{
			name:      "QueryFails_ReturnsError",
			replicas:  []string{"host-e:9000"},
			queryErr:  errors.New("code: 279, message: All connection tries failed"),
			wantErr:   ErrFailedToWaitForMutations,
			wantCalls: [][]any{{"cluster-a", replicaID(1)}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var retry *backoff.ExponentialBackOff
			if testCase.retry != nil {
				retry = testCase.retry()
			}
			m := newMockManager(t, "cluster-a", retry)
			m.expectReplicas("cluster-a", testCase.replicas...)
			for i := range testCase.replicas {
				if testCase.queryErr != nil {
					m.mock.ExpectSelect(mutationsQuery).WillReturnError(testCase.queryErr)
					break
				}
				for range testCase.pendingPolls[replicaID(i+1)] {
					m.mock.ExpectSelect(mutationsQuery).WillReturnRows(mutationRows(1))
				}
				m.mock.ExpectSelect(mutationsQuery).WillReturnRows(mutationRows(0))
			}

			err := m.manager.WaitForRunningMutations(context.Background())

			if testCase.wantErr != nil {
				assert.ErrorIs(t, err, testCase.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.wantCalls, m.recorder.selectCalls)
			assert.NoError(t, m.mock.ExpectationsWereMet())
		})
	}
}

func TestDevelopmentModeSkipsReplicaChecks(t *testing.T) {
	m := newMockManager(t, "cluster-c", nil, WithDevelopment(true))

	assert.NoError(t, m.manager.WaitForRunningMutations(context.Background()))
	assert.Empty(t, m.recorder.selectCalls)
	assert.NoError(t, m.mock.ExpectationsWereMet())
}

func replicaID(n int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", n)
}
