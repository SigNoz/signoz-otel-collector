package schemamigrator

import (
	"testing"
)

func TestTableEngine(t *testing.T) {
	testCases := []struct {
		name    string
		op      TableEngine
		wantSQL string
	}{
		{
			name: "create table",
			op: ReplacingMergeTree{
				MergeTree{
					OrderBy: "(timestamp, resource_id)",
				},
			},
			wantSQL: "ReplacingMergeTree ORDER BY (timestamp, resource_id)",
		},
		{
			name: "create table with distributed engine",
			op: Distributed{
				Database:    "default",
				Table:       "logs",
				Cluster:     "cluster",
				ShardingKey: "rand()",
			},
			wantSQL: "Distributed('cluster', default, logs, rand())",
		},
	}

	for _, tc := range testCases {
		gotSQL := tc.op.ToSQL()
		if gotSQL != tc.wantSQL {
			t.Errorf("got %s, want %s", gotSQL, tc.wantSQL)
		}
	}
}
