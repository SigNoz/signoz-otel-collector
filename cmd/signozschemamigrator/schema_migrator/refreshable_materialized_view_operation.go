package schemamigrator

import (
	"strings"
)

type CreateRefreshableMaterializedViewOperation struct {
	cluster   string
	Database  string
	ViewName  string
	DestTable string
	Columns   []Column

	RefreshInterval      string
	RandomizeForInterval string
	Append               bool

	Query string
}

func (c CreateRefreshableMaterializedViewOperation) OnCluster(cluster string) Operation {
	c.cluster = cluster
	return &c
}

func (c CreateRefreshableMaterializedViewOperation) WithReplication() Operation {
	return &c
}

func (c CreateRefreshableMaterializedViewOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, c.Database, c.ViewName
}

func (c CreateRefreshableMaterializedViewOperation) IsMutation() bool { return false }

func (c CreateRefreshableMaterializedViewOperation) IsIdempotent() bool { return true }

func (c CreateRefreshableMaterializedViewOperation) IsLightweight() bool { return true }

func (c CreateRefreshableMaterializedViewOperation) ForceMigrate() bool { return false }

func (c CreateRefreshableMaterializedViewOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("CREATE MATERIALIZED VIEW IF NOT EXISTS ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.ViewName)
	if c.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(c.cluster)
	}
	// REFRESH clause comes immediately after the view name (+ ON CLUSTER).
	sql.WriteString(" REFRESH EVERY ")
	sql.WriteString(c.RefreshInterval)
	if c.RandomizeForInterval != "" {
		sql.WriteString(" RANDOMIZE FOR ")
		sql.WriteString(c.RandomizeForInterval)
	}
	if c.Append {
		sql.WriteString(" APPEND")
	}
	sql.WriteString(" TO ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.DestTable)
	if len(c.Columns) > 0 {
		columnParts := []string{}
		for _, column := range c.Columns {
			columnParts = append(columnParts, column.ToSQL())
		}
		sql.WriteString(" (")
		sql.WriteString(strings.Join(columnParts, ", "))
		sql.WriteString(")")
	}
	sql.WriteString(" AS ")
	sql.WriteString(c.Query)
	return sql.String()
}
