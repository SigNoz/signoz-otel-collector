package schemamigrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateProjectionOperation(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "create-projection",
			op: CreateProjectionOperation{
				Database: "db",
				Table:    "table",
				Projection: Projection{
					Name:  "projection",
					Query: "SELECT * order by timestamp",
				},
			},
			want: "ALTER TABLE db.table ADD PROJECTION IF NOT EXISTS projection (SELECT * order by timestamp)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestDropProjectionOperation(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "drop-projection",
			op: DropProjectionOperation{
				Database: "db",
				Table:    "table",
				Projection: Projection{
					Name: "projection",
				},
			},
			want: "ALTER TABLE db.table DROP PROJECTION IF EXISTS projection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}

}
