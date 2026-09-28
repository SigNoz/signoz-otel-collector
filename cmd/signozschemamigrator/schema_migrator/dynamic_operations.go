package schemamigrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// DynamicOperation is an Operation whose concrete operations depend on the
// database state and are resolved when it runs.
type DynamicOperation interface {
	Operation
	Resolve(ctx context.Context, conn clickhouse.Conn) ([]Operation, error)
}

// MaterializedKeyIndexes resolves to one index on JSONColumn per key
// materialized from MapColumn, i.e. every column of Table whose DEFAULT or
// MATERIALIZED expression is exactly MapColumn['key'].
//
// The index expression is JSONColumn.`key`::String, the form the query
// builder reads string attributes with; any other form is not used.
type MaterializedKeyIndexes struct {
	Database    string
	Table       string
	MapColumn   string
	JSONColumn  string
	IndexType   string // ex: ngrambf_v1(4, 5000, 2, 0)
	Granularity int
	// Drop resolves to dropping the indexes instead.
	Drop bool
}

func (o MaterializedKeyIndexes) OnCluster(string) Operation { return &o }

func (o MaterializedKeyIndexes) WithReplication() Operation { return &o }

func (o MaterializedKeyIndexes) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, o.Database, o.Table
}

func (o MaterializedKeyIndexes) IsMutation() bool { return false }

func (o MaterializedKeyIndexes) IsIdempotent() bool { return true }

func (o MaterializedKeyIndexes) IsLightweight() bool { return true }

func (o MaterializedKeyIndexes) ForceMigrate() bool { return false }

// ToSQL returns the query listing the materialized keys.
func (o MaterializedKeyIndexes) ToSQL() string {
	return fmt.Sprintf(
		"SELECT DISTINCT extract(default_expression, '^%s\\\\[\\'(.+)\\'\\\\]$') AS key FROM system.columns"+
			" WHERE database = '%s' AND table = '%s' AND default_kind IN ('DEFAULT', 'MATERIALIZED') AND key != '' ORDER BY key",
		o.MapColumn, o.Database, o.Table,
	)
}

func (o MaterializedKeyIndexes) Resolve(ctx context.Context, conn clickhouse.Conn) ([]Operation, error) {
	rows, err := conn.Query(ctx, o.ToSQL())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ops []Operation
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		ops = append(ops, o.indexOperation(key))
	}
	return ops, rows.Err()
}

func (o MaterializedKeyIndexes) indexOperation(key string) Operation {
	indexType, _, _ := strings.Cut(o.IndexType, "(")
	index := Index{
		Name:        quoteIdentifier(fmt.Sprintf("%s.%s_String_%s", o.JSONColumn, key, indexType)),
		Expression:  fmt.Sprintf("%s.%s::String", o.JSONColumn, quoteIdentifier(key)),
		Type:        o.IndexType,
		Granularity: o.Granularity,
	}
	if o.Drop {
		return AlterTableDropIndex{Database: o.Database, Table: o.Table, Index: index}
	}
	return AlterTableAddIndex{Database: o.Database, Table: o.Table, Index: index}
}

func quoteIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "\\`") + "`"
}
