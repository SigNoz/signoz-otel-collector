package schemamigrator

import (
	"strings"
)

// AlterTableAddColumn is used to add a column to a table.
// It is used to represent the ALTER TABLE ADD COLUMN statement in the SQL.
type AlterTableAddColumn struct {
	cluster string

	Database string
	Table    string
	Column   Column
	// Should be used carefully, this is to be used when the column is to be added after a specific column
	// If not specified, the column will be added at the end
	After *Column
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableAddColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableAddColumn) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableAddColumn) ForceMigrate() bool {
	return false
}

func (a AlterTableAddColumn) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableAddColumn) IsMutation() bool {
	// Adding a column is not a mutation. It simply updates the metadata of the table.
	return false
}

func (a AlterTableAddColumn) IsIdempotent() bool {
	// Adding a column is idempotent. It will not change the table if the column already exists.
	return true
}

func (a AlterTableAddColumn) IsLightweight() bool {
	// Adding a column is lightweight. It simply updates the metadata of the table.
	return true
}

func (a AlterTableAddColumn) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" ADD COLUMN IF NOT EXISTS ")
	sql.WriteString(a.Column.ToSQL())
	if a.After != nil {
		sql.WriteString(" AFTER ")
		sql.WriteString(a.After.Name)
	}
	return sql.String()
}

// AlterTableDropColumn is used to drop a column from a table.
// It is used to represent the ALTER TABLE DROP COLUMN statement in the SQL.
type AlterTableDropColumn struct {
	cluster  string
	Database string
	Table    string
	Column   Column
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableDropColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableDropColumn) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableDropColumn) ShouldWaitForDistributionQueue() (bool, string, string) {
	return true, a.Database, a.Table
}

func (a AlterTableDropColumn) IsMutation() bool {
	// Dropping a column is a mutation. It will remove the column from the table.
	return true
}

func (a AlterTableDropColumn) IsIdempotent() bool {
	// Dropping a column is idempotent. It will not change the table if the column does not exist.
	return true
}

func (a AlterTableDropColumn) IsLightweight() bool {
	// Dropping a column is lightweight. It removes the data from the disk
	// which is a lightweight operation.
	return true
}

func (a AlterTableDropColumn) ForceMigrate() bool {
	return false
}

func (a AlterTableDropColumn) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" DROP COLUMN IF EXISTS ")
	sql.WriteString(a.Column.Name)
	return sql.String()
}

// AlterTableModifyColumn is used to modify a column in a table.
// It is used to represent the ALTER TABLE MODIFY COLUMN statement in the SQL.
type AlterTableModifyColumn struct {
	cluster string

	Database string
	Table    string
	Column   Column
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableModifyColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumn) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifyColumn) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifyColumn) IsMutation() bool {
	// If the column type or ttl is modified, it is a mutation.
	// This is because the column data will be re-written.
	return a.Column.Type != nil || a.Column.TTL != ""
}

func (a AlterTableModifyColumn) IsIdempotent() bool {
	// Modifying a column is idempotent. It will not change the table if the column does not exist.
	return true
}

func (a AlterTableModifyColumn) IsLightweight() bool {
	// If the column type or ttl is modified, it is a mutation that
	// re-writes the column data.
	return a.Column.Type != nil || a.Column.TTL != ""
}

func (a AlterTableModifyColumn) ForceMigrate() bool {
	return false
}

func (a AlterTableModifyColumn) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY COLUMN IF EXISTS ")
	sql.WriteString(a.Column.Name)
	if a.Column.Type != nil {
		sql.WriteString(" ")
		sql.WriteString(a.Column.Type.String())
	}
	if a.Column.Default != "" {
		sql.WriteString(" DEFAULT ")
		sql.WriteString(a.Column.Default)
	}
	if a.Column.Codec != "" {
		sql.WriteString(" CODEC(")
		sql.WriteString(a.Column.Codec)
		sql.WriteString(")")
	}
	if a.Column.TTL != "" {
		sql.WriteString(" TTL ")
		sql.WriteString(a.Column.TTL)
	}
	if a.Column.Settings != nil {
		sql.WriteString(" SETTINGS ")
		sql.WriteString(a.Column.Settings.String())
	}
	return sql.String()
}

// AlterTableModifyColumnRemove is used to remove one of the column properties
// See https://clickhouse.com/docs/en/sql-reference/statements/alter/column#modify-column-remove
type AlterTableModifyColumnRemove struct {
	cluster string

	Database string
	Table    string
	Column   Column
	Property ColumnProperty
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableModifyColumnRemove) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnRemove) ForceMigrate() bool {
	return false
}

// WithReplication is a no-op for this operation.
func (a AlterTableModifyColumnRemove) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifyColumnRemove) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifyColumnRemove) IsMutation() bool {
	// Removing a column property is not a mutation. It simply updates the metadata of the table.
	return false
}

func (a AlterTableModifyColumnRemove) IsIdempotent() bool {
	// Removing a column property is idempotent. It will not change the table if the property does not exist.
	return false
}

func (a AlterTableModifyColumnRemove) IsLightweight() bool {
	// Removing a column property is lightweight. It simply updates the metadata of the table.
	return true
}

func (a AlterTableModifyColumnRemove) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY COLUMN IF EXISTS ")
	sql.WriteString(a.Column.Name)
	sql.WriteString(" REMOVE ")
	sql.WriteString(string(a.Property))
	return sql.String()
}

// AlterTableModifyColumnModifySettings is used to modify the settings of a column.
// It is used to represent the ALTER TABLE MODIFY COLUMN SETTINGS statement in the SQL.
type AlterTableModifyColumnModifySettings struct {
	cluster string

	Database string
	Table    string
	Column   Column
	Settings ColumnSettings
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableModifyColumnModifySettings) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnModifySettings) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifyColumnModifySettings) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifyColumnModifySettings) IsMutation() bool {
	// Modifying the settings of a column is not a mutation. It simply updates the metadata of the table.
	return false
}

func (a AlterTableModifyColumnModifySettings) IsIdempotent() bool {
	// Modifying the settings of a column is idempotent. It will not change the table if the settings do not exist.
	return true
}

func (a AlterTableModifyColumnModifySettings) IsLightweight() bool {
	// Modifying the settings of a column is lightweight. It simply updates the metadata of the table.
	return true
}

func (a AlterTableModifyColumnModifySettings) ForceMigrate() bool {
	return false
}

func (a AlterTableModifyColumnModifySettings) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY COLUMN IF EXISTS ")
	sql.WriteString(a.Column.Name)
	sql.WriteString(" MODIFY SETTING ")
	sql.WriteString(a.Settings.String())
	return sql.String()
}

// AlterTableModifyColumnResetSettings is used to reset the settings of a column.
// It is used to represent the ALTER TABLE MODIFY COLUMN RESET SETTINGS statement in the SQL.
type AlterTableModifyColumnResetSettings struct {
	cluster string

	Database string
	Table    string
	Column   Column
	Settings ColumnSettings
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableModifyColumnResetSettings) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnResetSettings) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableModifyColumnResetSettings) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableModifyColumnResetSettings) IsMutation() bool {
	// Resetting the settings of a column is not a mutation. It simply updates the metadata of the table.
	return false
}

func (a AlterTableModifyColumnResetSettings) IsIdempotent() bool {
	// Resetting the settings of a column is idempotent. It will not change the table if the settings do not exist.
	return true
}

func (a AlterTableModifyColumnResetSettings) IsLightweight() bool {
	// Resetting the settings of a column is lightweight. It simply updates the metadata of the table.
	return true
}

func (a AlterTableModifyColumnResetSettings) ForceMigrate() bool {
	return false
}

func (a AlterTableModifyColumnResetSettings) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MODIFY COLUMN IF EXISTS ")
	sql.WriteString(a.Column.Name)
	sql.WriteString(" RESET SETTING ")
	sql.WriteString(strings.Join(a.Settings.Names(), ", "))
	return sql.String()
}

// AlterTableMaterializeColumn is used to materialize a column.
// It is used to represent the ALTER TABLE MATERIALIZE COLUMN statement in the SQL.
type AlterTableMaterializeColumn struct {
	cluster string

	Database    string
	Table       string
	Column      Column
	Partition   string
	PartitionID string
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (a AlterTableMaterializeColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableMaterializeColumn) WithReplication() Operation {
	// no-op
	return &a
}

func (a AlterTableMaterializeColumn) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, a.Database, a.Table
}

func (a AlterTableMaterializeColumn) IsMutation() bool {
	// Materializing a column is a mutation. It will create a new column.
	return true
}

func (a AlterTableMaterializeColumn) IsIdempotent() bool {
	// Materializing a column is idempotent. It will not change the table if the column already exists.
	return true
}

func (a AlterTableMaterializeColumn) IsLightweight() bool {
	// Materializing a column is not lightweight. It will rewrite the column data with materialized data.
	return false
}

func (a AlterTableMaterializeColumn) ForceMigrate() bool {
	return false
}

func (a AlterTableMaterializeColumn) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(a.Database)
	sql.WriteString(".")
	sql.WriteString(a.Table)
	if a.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(a.cluster)
	}
	sql.WriteString(" MATERIALIZE COLUMN ")
	sql.WriteString(a.Column.Name)
	if a.Partition != "" {
		sql.WriteString(" IN PARTITION ")
		sql.WriteString(a.Partition)
	} else if a.PartitionID != "" {
		sql.WriteString(" IN PARTITION ")
		sql.WriteString(a.PartitionID)
	}
	return sql.String()
}
