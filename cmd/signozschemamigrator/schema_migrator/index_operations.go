package schemamigrator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
)

type IndexType string

const (
	IndexTypeTokenBF IndexType = "tokenbf_v1"
	IndexTypeNGramBF IndexType = "ngrambf_v1"
	IndexTypeMinMax  IndexType = "minmax"
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

func JSONSubColumnIndexName(column, path, typeColumn string, index IndexType) string {
	expr := column + "." + path
	return fmt.Sprintf("`%s_%s_%s`", expr, typeColumn, index)
}

func jsonSubColumnIndexExprFormat(expr, typeColumn string) string {
	parts := strings.Split(expr, ".")
	for idx, part := range parts {
		if keycheck.IsBacktickRequired(part) {
			part := strings.Trim(part, "`") // trim if already present
			parts[idx] = "`" + part + "`"
		}
	}
	return fmt.Sprintf("lower(assumeNotNull(dynamicElement(%s, '%s')))", strings.Join(parts, "."), typeColumn)
}

func JSONSubColumnIndexExpr(column, path, typeColumn string) string {
	expr := column + "." + path
	return jsonSubColumnIndexExprFormat(expr, typeColumn)
}

// Returns the subcolumn name from the index expression
// If the expression is not a JSON subcolumn index expression, returns an error
func UnfoldJSONSubColumnIndexExpr(expr string) (string, string, error) {
	if !strings.HasPrefix(expr, "lower(assumeNotNull(dynamicElement(") {
		return "", "", fmt.Errorf("invalid expression: %s, expected prefix: lower(assumeNotNull(dynamicElement(...))", expr)
	}

	// remove lower()
	expr = strings.TrimPrefix(expr, "lower(")
	expr = strings.TrimSuffix(expr, ")")

	// remove assumeNotNull()
	expr = strings.TrimPrefix(expr, "assumeNotNull(")
	expr = strings.TrimSuffix(expr, ")")

	// remove dynamicElement()
	expr = strings.TrimPrefix(expr, "dynamicElement(")
	expr = strings.TrimSuffix(expr, ")")

	// split by comma
	parts := strings.Split(expr, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid expression: %s", expr)
	}

	expr = parts[0]

	// extract the column type and trim the quotes
	typeColumn := strings.TrimSpace(parts[1])

	// trim the type column
	typeColumn = strings.TrimLeft(typeColumn, "'")
	typeColumn = strings.TrimRight(typeColumn, "'")

	return expr, typeColumn, nil
}
