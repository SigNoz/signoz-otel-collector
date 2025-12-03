package schemamigrator

import (
	"strconv"
	"strings"
)

// Index is used to represent an index in the SQL.
type Index struct {
	Name        string // name of the index; ex: idx_name
	Expression  string // expression of the index; ex: traceID
	Type        string // type of the index; ex: tokenbf_v1(1024, 2, 0)
	Granularity int    // granularity of the index; ex: 1
}

func (i Index) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("INDEX ")
	sql.WriteString(i.Name)
	sql.WriteString(" ")
	sql.WriteString(i.Expression)
	sql.WriteString(" TYPE ")
	sql.WriteString(i.Type)
	sql.WriteString(" GRANULARITY ")
	sql.WriteString(strconv.Itoa(i.Granularity))
	return sql.String()
}

// AlterTableAddIndex is used to add an index to a table.
// It is used to represent the ALTER TABLE ADD INDEX statement in the SQL.
type AlterTableAddIndex struct {
	cluster string

	Database string
	Table    string
	Index    Index
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableAddIndex) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableAddIndex) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableAddIndex) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableAddIndex) IsMutation() bool {
	// Adding an index is not a mutation. It will create a new index.
	return false
}

func (a AlterTableAddIndex) IsIdempotent() bool {
	// Adding an index is idempotent. It will not change the table if the index already exists.
	return true
}

func (a AlterTableAddIndex) IsLightweight() bool {
	// Adding an index is lightweight. It will create a new index for the new data.
	return true
}

func (a AlterTableAddIndex) ToSQL() string {
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
	sql.WriteString(" ")
	sql.WriteString(a.Index.Expression)
	sql.WriteString(" TYPE ")
	sql.WriteString(a.Index.Type)
	sql.WriteString(" GRANULARITY ")
	sql.WriteString(strconv.Itoa(a.Index.Granularity))
	return sql.String()
}

// AlterTableDropIndex is used to drop an index from a table.
// It is used to represent the ALTER TABLE DROP INDEX statement in the SQL.
type AlterTableDropIndex struct {
	cluster  string
	Database string
	Table    string
	Index    Index
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableDropIndex) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableDropIndex) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableDropIndex) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableDropIndex) IsMutation() bool {
	// Dropping an index is a mutation. It will remove the index from the table.
	return true
}

func (a AlterTableDropIndex) IsIdempotent() bool {
	// Dropping an index is idempotent. It will not change the table if the index does not exist.
	return true
}

func (a AlterTableDropIndex) IsLightweight() bool {
	// Dropping an index is lightweight. It will remove the index from the table.
	return true
}

func (a AlterTableDropIndex) ToSQL() string {
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

// AlterTableMaterializeIndex is used to materialize an index on a table.
// It is used to represent the ALTER TABLE MATERIALIZE INDEX statement in the SQL.
type AlterTableMaterializeIndex struct {
	cluster   string
	Database  string
	Table     string
	Index     Index
	Partition string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableMaterializeIndex) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableMaterializeIndex) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableMaterializeIndex) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableMaterializeIndex) IsMutation() bool {
	// Materializing an index is a mutation. It will create a new index for the new data.
	return true
}

func (a AlterTableMaterializeIndex) IsIdempotent() bool {
	// Materializing an index is idempotent. It will not change the table if the index already exists.
	return true
}

func (a AlterTableMaterializeIndex) IsLightweight() bool {
	// Materializing an index is not lightweight. It will create a complete index for all the data in the table.
	return false
}

func (a AlterTableMaterializeIndex) ToSQL() string {
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

// AlterTableClearIndex is used to clear an index from a table.
// It is used to represent the ALTER TABLE CLEAR INDEX statement in the SQL.
type AlterTableClearIndex struct {
	cluster   string
	Database  string
	Table     string
	Index     Index
	Partition string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableClearIndex) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableClearIndex) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableClearIndex) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableClearIndex) IsMutation() bool {
	// Clearing an index is a mutation. It will remove the index from the table.
	return true
}

func (a AlterTableClearIndex) IsIdempotent() bool {
	// Clearing an index is idempotent. It will not change the table if the index does not exist.
	return true
}

func (a AlterTableClearIndex) IsLightweight() bool {
	// Clearing an index is not lightweight. It will remove the index from the table.
	return false
}

func (a AlterTableClearIndex) ToSQL() string {
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
