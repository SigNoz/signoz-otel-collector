package schemamigrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInsertIntoTable(t *testing.T) {
	timestamp := time.Now().UnixMilli()
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "insert-into-table",
			op: InsertIntoTable{
				Database: "signoz_logs",
				Table:    "distributed_promoted_paths",
				Columns:  []string{"path", "created_at"},
				Values: [][]any{
					{"message", timestamp},
				},
			},
			want: fmt.Sprintf("INSERT INTO signoz_logs.distributed_promoted_paths (path, created_at) VALUES ('message', %d)", timestamp),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
