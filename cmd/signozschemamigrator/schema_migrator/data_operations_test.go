package schemamigrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/constants"
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
				Database: constants.SignozMetadataDB,
				Table:    constants.DistributedPromotedPathsTable,
				Columns:  []string{"path", "created_at"},
				Values:   fmt.Sprintf("('message', %d)", timestamp),
			},
			want: fmt.Sprintf("INSERT INTO %s.%s (path, created_at) VALUES ('message', %d)", constants.SignozMetadataDB, constants.DistributedPromotedPathsTable, timestamp),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
