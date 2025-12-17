package schemamigrator

import "strings"

type CreateProjectionOperation struct {
	cluster    string
	Database   string
	Table      string
	Projection Projection
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (c CreateProjectionOperation) OnCluster(cluster string) Operation {
	c.cluster = cluster
	return &c
}

func (c CreateProjectionOperation) WithReplication() Operation {
	// no-op
	return &c
}

func (c CreateProjectionOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, c.Database, c.Table
}

func (c CreateProjectionOperation) IsMutation() bool {
	// Create projection is not a mutation.
	return false
}

func (c CreateProjectionOperation) IsIdempotent() bool {
	// Create projection is idempotent. It will not change the table if the projection already exists.
	return true
}

func (c CreateProjectionOperation) IsLightweight() bool {
	// Create projection is lightweight.
	return true
}

func (c CreateProjectionOperation) ForceMigrate() bool {
	return false
}

func (c CreateProjectionOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(c.Database)
	sql.WriteString(".")
	sql.WriteString(c.Table)
	sql.WriteString(" ADD PROJECTION IF NOT EXISTS ")
	sql.WriteString(c.Projection.Name)
	sql.WriteString(" (")
	sql.WriteString(c.Projection.Query)
	sql.WriteString(")")
	return sql.String()
}

type DropProjectionOperation struct {
	cluster    string
	Database   string
	Table      string
	Projection Projection
}

// OnCluster is used to specify the cluster on which the operation should be performed.
// This is useful when the operation is to be performed on a cluster setup.
func (d DropProjectionOperation) OnCluster(cluster string) Operation {
	d.cluster = cluster
	return &d
}

func (d DropProjectionOperation) WithReplication() Operation {
	// no-op
	return &d
}

func (d DropProjectionOperation) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, d.Database, d.Table
}

func (d DropProjectionOperation) IsMutation() bool {
	// Drop projection is not a mutation.
	return false
}

func (d DropProjectionOperation) IsIdempotent() bool {
	// Drop projection is idempotent. It will not change the table if the projection does not exist.
	return true
}

func (d DropProjectionOperation) IsLightweight() bool {
	// Drop projection is lightweight.
	return true
}

func (d DropProjectionOperation) ForceMigrate() bool {
	return false
}

func (d DropProjectionOperation) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("ALTER TABLE ")
	sql.WriteString(d.Database)
	sql.WriteString(".")
	sql.WriteString(d.Table)
	if d.cluster != "" {
		sql.WriteString(" ON CLUSTER ")
		sql.WriteString(d.cluster)
	}
	sql.WriteString(" DROP PROJECTION IF EXISTS ")
	sql.WriteString(d.Projection.Name)
	return sql.String()
}
