package main

import "strings"

type CreateTableOperation struct {
	cluster  string
	Database string
	Table    string
	Columns  []Column
	Engine   TableEngine
}

func (c *CreateTableOperation) OnCluster(cluster string) *CreateTableOperation {
	c.cluster = cluster
	return c
}

func (c *CreateTableOperation) IsMutation() bool {
	return false
}

func (c *CreateTableOperation) IsIdempotent() bool {
	return true
}

func (c *CreateTableOperation) IsLightweight() bool {
	return true
}

func (c *CreateTableOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.Table)
	if c.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(c.cluster)
	}
	columnParts := []string{}
	for _, column := range c.Columns {
		columnParts = append(columnParts, column.ToSQL())
	}
	sql.WriteString(" (")
	sql.WriteString(strings.Join(columnParts, ", "))
	sql.WriteString(")")
	sql.WriteString(" ENGINE = ")
	sql.WriteString(c.Engine.ToSQL())
	return sql.String()
}

type CreateMaterializedViewOperation struct {
	cluster   string
	Database  string
	ViewName  string
	DestTable string
	Query     string
}

func (c *CreateMaterializedViewOperation) OnCluster(cluster string) *CreateMaterializedViewOperation {
	c.cluster = cluster
	return c
}

func (c *CreateMaterializedViewOperation) IsMutation() bool {
	return false
}

func (c *CreateMaterializedViewOperation) IsIdempotent() bool {
	return true
}

func (c *CreateMaterializedViewOperation) IsLightweight() bool {
	return true
}

func (c *CreateMaterializedViewOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("CREATE MATERIALIZED VIEW ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.ViewName)
	if c.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(c.cluster)
	}
	sql.WriteString(" TO ")
	sql.WriteString(c.DestTable)
	sql.WriteString(" AS ")
	sql.WriteString(c.Query)
	return sql.String()
}
