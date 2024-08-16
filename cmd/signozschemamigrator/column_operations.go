package main

import "strings"

type AlterTableAddColumn struct {
	cluster string

	Database string
	Table    string
	Column   Column
	// Should be used carefully, this is to be used when the column is to be added after a specific column
	// If not specified, the column will be added at the end
	After *Column
}

func (a AlterTableAddColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableAddColumn) IsMutation() bool {
	return false
}

func (a AlterTableAddColumn) IsIdempotent() bool {
	return true
}

func (a AlterTableAddColumn) IsLightweight() bool {
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

type AlterTableDropColumn struct {
	cluster  string
	Database string
	Table    string
	Column   Column
}

func (a AlterTableDropColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableDropColumn) IsMutation() bool {
	return true
}

func (a AlterTableDropColumn) IsIdempotent() bool {
	return true
}

func (a AlterTableDropColumn) IsLightweight() bool {
	return true
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

type AlterTableModifyColumn struct {
	cluster string

	Database string
	Table    string
	Column   Column
}

func (a AlterTableModifyColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumn) IsMutation() bool {
	// If the column type or ttl is modified, it is a mutation.
	return a.Column.Type != nil || a.Column.TTL != ""
}

func (a AlterTableModifyColumn) IsIdempotent() bool {
	return true
}

func (a AlterTableModifyColumn) IsLightweight() bool {
	// If the column type or ttl is modified, it is a mutation that
	// re-writes the column data.
	return a.Column.Type != nil || a.Column.TTL != ""
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
		sql.WriteString(a.Column.Type.String())
	}
	if a.Column.Default != "" {
		sql.WriteString(" DEFAULT ")
		sql.WriteString(a.Column.Default)
	}
	if a.Column.Codec != "" {
		sql.WriteString(" CODEC ")
		sql.WriteString(a.Column.Codec)
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

func (a AlterTableModifyColumnRemove) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnRemove) IsMutation() bool {
	return false
}

func (a AlterTableModifyColumnRemove) IsIdempotent() bool {
	return false
}

func (a AlterTableModifyColumnRemove) IsLightweight() bool {
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

type AlterTableModifyColumnModifySettings struct {
	cluster string

	Database string
	Table    string
	Column   Column
	Settings ColumnSettings
}

func (a AlterTableModifyColumnModifySettings) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnModifySettings) IsMutation() bool {
	return false
}

func (a AlterTableModifyColumnModifySettings) IsIdempotent() bool {
	return true
}

func (a AlterTableModifyColumnModifySettings) IsLightweight() bool {
	return true
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

type AlterTableModifyColumnResetSettings struct {
	cluster string

	Database string
	Table    string
	Column   Column
	Settings ColumnSettings
}

func (a AlterTableModifyColumnResetSettings) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableModifyColumnResetSettings) IsMutation() bool {
	return false
}

func (a AlterTableModifyColumnResetSettings) IsIdempotent() bool {
	return true
}

func (a AlterTableModifyColumnResetSettings) IsLightweight() bool {
	return true
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

type AlterTableMaterializeColumn struct {
	cluster string

	Database    string
	Table       string
	Column      Column
	Partition   string
	PartitionID string
}

func (a AlterTableMaterializeColumn) OnCluster(cluster string) Operation {
	a.cluster = cluster
	return &a
}

func (a AlterTableMaterializeColumn) IsMutation() bool {
	return true
}

func (a AlterTableMaterializeColumn) IsIdempotent() bool {
	return true
}

func (a AlterTableMaterializeColumn) IsLightweight() bool {
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
