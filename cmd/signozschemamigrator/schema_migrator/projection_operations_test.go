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
		{
			name: "create-projection-on-cluster",
			op: CreateProjectionOperation{
				Database: "db",
				Table:    "table",
				Projection: Projection{
					Name:  "projection",
					Query: "SELECT * order by timestamp",
				},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster ADD PROJECTION IF NOT EXISTS projection (SELECT * order by timestamp)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
		})
	}
}

func TestAlterTableMaterializeProjection(t *testing.T) {
	testCases := []struct {
		name string
		op   Operation
		want string
	}{
		{
			name: "materialize-projection",
			op: AlterTableMaterializeProjection{
				Database:   "db",
				Table:      "table",
				Projection: Projection{Name: "projection"},
			},
			want: "ALTER TABLE db.table MATERIALIZE PROJECTION IF EXISTS projection",
		},
		{
			name: "materialize-projection-on-cluster",
			op: AlterTableMaterializeProjection{
				Database:   "db",
				Table:      "table",
				Projection: Projection{Name: "projection"},
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MATERIALIZE PROJECTION IF EXISTS projection",
		},
		{
			name: "materialize-projection-in-partition",
			op: AlterTableMaterializeProjection{
				Database:   "db",
				Table:      "table",
				Projection: Projection{Name: "projection"},
				Partition:  "'2026-10-01'",
			}.OnCluster("cluster"),
			want: "ALTER TABLE db.table ON CLUSTER cluster MATERIALIZE PROJECTION IF EXISTS projection IN PARTITION '2026-10-01'",
		},
	}

	manager := newTestMigrationManager(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.op.ToSQL())
			require.Equal(t, tc.want, tc.op.WithReplication().ToSQL())
			require.True(t, tc.op.IsMutation())
			require.True(t, tc.op.IsIdempotent())
			require.False(t, tc.op.IsLightweight())
			require.False(t, tc.op.ForceMigrate())
			require.False(t, manager.IsSyncOperation(tc.op))
			require.True(t, manager.IsAsyncOperation(tc.op))
			wait, database, table := tc.op.ShouldWaitForDistributionQueue()
			require.False(t, wait)
			require.Equal(t, "db", database)
			require.Equal(t, "table", table)
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
