package main

import (
	"strconv"
	"strings"
)

type Index struct {
	Name        string // name of the index; ex: idx_name
	Expression  string // expression of the index; ex: traceID
	Type        string // type of the index; ex: tokenbf_v1(1024, 2, 0)
	Granularity int    // granularity of the index; ex: 1
}

type AlterTableAddIndex struct {
	cluster string

	Database string
	Table    string
	Index    Index
}

func (a *AlterTableAddIndex) OnCluster(cluster string) *AlterTableAddIndex {
	a.cluster = cluster
	return a
}

func (a *AlterTableAddIndex) IsMutation() bool {
	return false
}

func (a *AlterTableAddIndex) IsIdempotent() bool {
	return true
}

func (a *AlterTableAddIndex) IsLightweight() bool {
	return true
}

func (a *AlterTableAddIndex) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" ADD INDEX IF NOT EXISTS ")
	sql.WriteString(a.Index.Name)
	sql.WriteString(" (")
	sql.WriteString(a.Index.Expression)
	sql.WriteString(")")
	sql.WriteString(" TYPE ")
	sql.WriteString(a.Index.Type)
	sql.WriteString(" GRANULARITY ")
	sql.WriteString(strconv.Itoa(a.Index.Granularity))
	return sql.String()
}

type AlterTableDropIndex struct {
	cluster  string
	Database string
	Table    string
	Index    Index
}

func (a *AlterTableDropIndex) OnCluster(cluster string) *AlterTableDropIndex {
	a.cluster = cluster
	return a
}

func (a *AlterTableDropIndex) IsMutation() bool {
	return true
}

func (a *AlterTableDropIndex) IsIdempotent() bool {
	return true
}

func (a *AlterTableDropIndex) IsLightweight() bool {
	return true
}

func (a *AlterTableDropIndex) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" DROP INDEX IF EXISTS ")
	sql.WriteString(a.Index.Name)
	return sql.String()
}

type AlterTableMaterializeIndex struct {
	cluster   string
	Database  string
	Table     string
	Index     Index
	Partition string
}

func (a *AlterTableMaterializeIndex) OnCluster(cluster string) *AlterTableMaterializeIndex {
	a.cluster = cluster
	return a
}

func (a *AlterTableMaterializeIndex) IsMutation() bool {
	return true
}

func (a *AlterTableMaterializeIndex) IsIdempotent() bool {
	return true
}

func (a *AlterTableMaterializeIndex) IsLightweight() bool {
	return false
}

func (a *AlterTableMaterializeIndex) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MATERIALIZE INDEX IF EXISTS ")
	sql.WriteString(a.Index.Name)
	if a.Partition != "" {
		sql.WriteString(" IN PARTITION ")
		sql.WriteString(a.Partition)
	}
	return sql.String()
}

type AlterTableClearIndex struct {
	cluster   string
	Database  string
	Table     string
	Index     Index
	Partition string
}

func (a *AlterTableClearIndex) OnCluster(cluster string) *AlterTableClearIndex {
	a.cluster = cluster
	return a
}

func (a *AlterTableClearIndex) IsMutation() bool {
	return true
}

func (a *AlterTableClearIndex) IsIdempotent() bool {
	return true
}

func (a *AlterTableClearIndex) IsLightweight() bool {
	return false
}

func (a *AlterTableClearIndex) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" CLEAR INDEX IF EXISTS ")
	sql.WriteString(a.Index.Name)
	if a.Partition != "" {
		sql.WriteString(" IN PARTITION ")
		sql.WriteString(a.Partition)
	}
	return sql.String()
}
