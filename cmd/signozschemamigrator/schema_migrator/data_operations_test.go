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
				Table:    "distributed_column_evolution_metadata",
				Columns:  []string{"signal", "column_name", "column_type", "field_context", "field_name", "version", "release_time"},
				Values:   fmt.Sprintf("('logs', 'body_json_promoted', 'JSON', 'body', 'message', 0, %d)", timestamp),
			},
			want: fmt.Sprintf("INSERT INTO %s.distributed_column_evolution_metadata (signal, column_name, column_type, field_context, field_name, version, release_time) VALUES ('logs', 'body_json_promoted', 'JSON', 'body', 'message', 0, %d)", constants.SignozMetadataDB, timestamp),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}
