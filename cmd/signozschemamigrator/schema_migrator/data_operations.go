package schemamigrator

import (
	"fmt"
	"strings"
)

// InsertIntoTable represents an INSERT INTO statement.
// Columns are provided as names; values are provided as Go values and encoded safely.
type InsertIntoTable struct {
	cluster string

	Database    string
	Table       string
	Columns     []string
	LightWeight bool
	Synchronous bool
	Values      [][]string
}

// OnCluster is a no-op for INSERTs (ClickHouse does not support INSERT ... ON CLUSTER).
func (i InsertIntoTable) OnCluster(cluster string) Operation {
	i.cluster = cluster
	return &i
}

// WithReplication is a no-op for this operation.
func (i InsertIntoTable) WithReplication() Operation {
	return &i
}

// ShouldWaitForDistributionQueue returns false as INSERTs do not start DDL queues.
func (i InsertIntoTable) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, i.Database, i.Table
}

// IsMutation returns false (does not alter existing data parts).
func (i InsertIntoTable) IsMutation() bool { return false }

func (i InsertIntoTable) IsIdempotent() bool { return true }

// IsLightweight returns true.
func (i InsertIntoTable) IsLightweight() bool { return i.LightWeight }

func (i InsertIntoTable) ForceMigrate() bool {
	return i.Synchronous
}

func (i InsertIntoTable) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("INSERT INTO ")
	sql.WriteString(i.Database)
	sql.WriteString(".")
	sql.WriteString(i.Table)

	if len(i.Columns) > 0 {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(i.Columns, ", "))
		sql.WriteString(")")
	}

	sql.WriteString(" VALUES ")

	rows := make([]string, 0, len(i.Values))
	for _, row := range i.Values {
		vals := make([]string, len(row))
		for idx, v := range row {
			vals[idx] = fmt.Sprintf("'%s'", v)
		}
		rows = append(rows, fmt.Sprintf("(%s)", strings.Join(vals, ", ")))
	}

	sql.WriteString(strings.Join(rows, ", "))
	return sql.String()
}
