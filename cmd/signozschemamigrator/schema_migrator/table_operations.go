package schemamigrator

import (
	"strings"
)

type Projection struct {
	Name  string
	Query string
}

func (p Projection) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("PROJECTION ")
	sql.WriteString(p.Name)
	sql.WriteString(" (")
	sql.WriteString(p.Query)
	sql.WriteString(")")
	return sql.String()
}

// CreateTableOperation is used to represent the CREATE TABLE statement in the SQL.
type CreateTableOperation struct {
	cluster     string
	Database    string
	Table       string
	Columns     []Column
	Indexes     []Index
	Projections []Projection
	Engine      TableEngine
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (c CreateTableOperation) OnCluster(cluster string) Operation {
	c.cluster = cluster
	c.Engine = c.Engine.OnCluster(cluster)
	return &c
}

func (c CreateTableOperation) WithReplication() Operation {
	c.Engine = c.Engine.WithReplication()
	return &c
}

func (c CreateTableOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, c.Database, c.Table
}

func (c CreateTableOperation) IsMutation() bool {
	// Create table is not a mutation.
	return false
}

func (c CreateTableOperation) IsIdempotent() bool {
	// Create table is idempotent. It will not change the table if the table already exists.
	return true
}

func (c CreateTableOperation) IsLightweight() bool {
	// Create table is lightweight.
	return true
}

func (c CreateTableOperation) ForceMigrate() bool {
	return false
}

func (c CreateTableOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE IF NOT EXISTS ")
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
	for _, index := range c.Indexes {
		sql.WriteString(", ")
		sql.WriteString(index.ToSQL())
	}
	for _, projection := range c.Projections {
		sql.WriteString(", ")
		sql.WriteString(projection.ToSQL())
	}
	sql.WriteString(")")
	sql.WriteString(" ENGINE = ")
	sql.WriteString(c.Engine.ToSQL())
	return sql.String()
}

type DropTableOperation struct {
	cluster  string
	Database string
	Table    string
	Settings TableSettings
}

func (d DropTableOperation) OnCluster(cluster string) Operation {
	d.cluster = cluster
	return &d
}

func (d DropTableOperation) WithReplication() Operation {
	// no-op
	return &d
}

func (d DropTableOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, d.Database, d.Table
}

func (d DropTableOperation) IsMutation() bool {
	return true
}

func (d DropTableOperation) IsIdempotent() bool {
	return true
}

func (d DropTableOperation) IsLightweight() bool {
	return true
}

func (d DropTableOperation) ForceMigrate() bool {
	return false
}

func (d DropTableOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("DROP TABLE IF EXISTS ")
	sql.WriteString(d.Database)
	sql.WriteString(".")
	sql.WriteString(d.Table)
	if d.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(d.cluster)
	}
	if len(d.Settings) > 0 {
		sql.WriteString(" SETTINGS ")
		sql.WriteString(d.Settings.ToSQL())
	}
	return sql.String()
}

// CreateMaterializedViewOperation is used to represent the CREATE MATERIALIZED VIEW statement in the SQL.
type CreateMaterializedViewOperation struct {
	cluster   string
	Database  string
	ViewName  string
	DestTable string
	Columns   []Column
	Query     string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (c CreateMaterializedViewOperation) OnCluster(cluster string) Operation {
	c.cluster = cluster
	return &c
}

func (c CreateMaterializedViewOperation) WithReplication() Operation {
	// no-op
	return &c
}

func (c CreateMaterializedViewOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, c.Database, c.ViewName
}

func (c CreateMaterializedViewOperation) IsMutation() bool {
	// Create materialized view is not a mutation.
	return false
}

func (c CreateMaterializedViewOperation) IsIdempotent() bool {
	// Create materialized view is idempotent. It will not change the materialized view if the materialized view already exists.
	return true
}

func (c CreateMaterializedViewOperation) IsLightweight() bool {
	// Create materialized view is lightweight.
	return true
}

func (c CreateMaterializedViewOperation) ForceMigrate() bool {
	return false
}

func (c CreateMaterializedViewOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("CREATE MATERIALIZED VIEW IF NOT EXISTS ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.ViewName)
	if c.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(c.cluster)
	}
	sql.WriteString(" TO ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.DestTable)
	if len(c.Columns) > 0 {
		sql.WriteString(" (")
		columnParts := []string{}
		for _, column := range c.Columns {
			columnParts = append(columnParts, column.ToSQL())
		}
		sql.WriteString(strings.Join(columnParts, ", "))
		sql.WriteString(")")
	}
	sql.WriteString(" AS ")
	sql.WriteString(c.Query)
	return sql.String()
}

// ModifyQueryMaterializedViewOperation is used to represent the ALTER TABLE ... MODIFY QUERY statement in the SQL.
type ModifyQueryMaterializedViewOperation struct {
	cluster  string
	Database string
	ViewName string
	Query    string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (c ModifyQueryMaterializedViewOperation) OnCluster(cluster string) Operation {
	c.cluster = cluster
	return &c
}

func (c ModifyQueryMaterializedViewOperation) WithReplication() Operation {
	// no-op
	return &c
}

func (c ModifyQueryMaterializedViewOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, c.Database, c.ViewName
}

func (c ModifyQueryMaterializedViewOperation) IsMutation() bool {
	// Modify materialized view is not a mutation.
	return false
}

func (c ModifyQueryMaterializedViewOperation) IsIdempotent() bool {
	// Modify materialized view is idempotent. It will not change the materialized view if the materialized view already exists.
	return true
}

func (c ModifyQueryMaterializedViewOperation) IsLightweight() bool {
	// Modify materialized view is lightweight.
	return true
}

func (c ModifyQueryMaterializedViewOperation) ForceMigrate() bool {
	return false
}

func (c ModifyQueryMaterializedViewOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.ViewName)
	if c.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(c.cluster)
	}
	sql.WriteString(" MODIFY QUERY ")
	sql.WriteString(c.Query)
	return sql.String()
}

type TruncateTableOperation struct {
	cluster  string
	Database string
	Table    string
}

func (d TruncateTableOperation) OnCluster(cluster string) Operation {
	d.cluster = cluster
	return &d
}

func (d TruncateTableOperation) WithReplication() Operation {
	// no-op
	return &d
}

func (d TruncateTableOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, d.Database, d.Table
}

func (d TruncateTableOperation) IsMutation() bool {
	return true
}

func (d TruncateTableOperation) IsIdempotent() bool {
	return true
}

func (d TruncateTableOperation) IsLightweight() bool {
	return false
}

func (d TruncateTableOperation) ForceMigrate() bool {
	return false
}

func (d TruncateTableOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("TRUNCATE TABLE IF EXISTS ")
	sql.WriteString(d.Database)
	sql.WriteString(".")
	sql.WriteString(d.Table)
	if d.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(d.cluster)
	}
	return sql.String()
}

type AlterTableModifyTTL struct {
	cluster string

	Database string
	Table    string
	TTL      string
	Settings ModifyTTLSettings
}

type ModifyTTLSettings struct {
	MaterializeTTLAfterModify bool
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableModifyTTL) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyTTL) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifyTTL) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifyTTL) IsMutation() bool {
	if !a.Settings.MaterializeTTLAfterModify {
		// we are not considering this as a mutation
		return false
	}

	return true
}

func (a AlterTableModifyTTL) IsIdempotent() bool {
	return true
}

func (a AlterTableModifyTTL) IsLightweight() bool {
	return !a.Settings.MaterializeTTLAfterModify
}

func (a AlterTableModifyTTL) ForceMigrate() bool {
	return false
}

func (a AlterTableModifyTTL) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY TTL ")
	sql.WriteString(a.TTL)
	if !a.Settings.MaterializeTTLAfterModify {
		sql.WriteString(" SETTINGS ")
		sql.WriteString("materialize_ttl_after_modify = 0")
	}
	return sql.String()
}

type AlterTableDropTTL struct {
	cluster  string
	Database string
	Table    string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableDropTTL) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableDropTTL) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableDropTTL) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableDropTTL) IsMutation() bool {
	return false
}

func (a AlterTableDropTTL) IsIdempotent() bool {
	return true
}

func (a AlterTableDropTTL) IsLightweight() bool {
	return true
}

func (a AlterTableDropTTL) ForceMigrate() bool {
	return false
}

func (a AlterTableDropTTL) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" REMOVE TTL")
	return sql.String()
}

// AlterTableModifySettings is used to modify table-level settings.
// It is used to represent the ALTER TABLE MODIFY SETTING statement in the SQL.
type AlterTableModifySettings struct {
	cluster string

	Database string
	Table    string
	Settings TableSettings
}

// OnCluster is used to specify the cluster on which the operation should be performed.
func (a AlterTableModifySettings) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifySettings) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifySettings) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifySettings) IsMutation() bool {
	// Modifying table settings is not a mutation. It simply updates the metadata of the table.
	// https://github.com/ClickHouse/ClickHouse/issues/86993#issuecomment-3280106638
	return false
}

func (a AlterTableModifySettings) IsIdempotent() bool {
	// Modifying table settings is idempotent.
	return true
}

func (a AlterTableModifySettings) IsLightweight() bool {
	// Modifying table settings is lightweight.
	return true
}

func (a AlterTableModifySettings) ForceMigrate() bool {
	return false
}

func (a AlterTableModifySettings) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY SETTING ")
	sql.WriteString(a.Settings.ToSQL())
	return sql.String()
}
