package schemamigrator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

// InsertIntoTable represents an INSERT INTO statement.
// Columns are provided as names; values are provided as Go values and encoded safely.
type InsertIntoTable struct {
	cluster string

	Database string
	Table    string
	Columns  []string
	Values   [][]any
}

// OnCluster is a no-op for INSERTs (ClickHouse does not support INSERT ... ON CLUSTER).
func (i InsertIntoTable) OnCluster(cluster string) Operation {
	i.cluster = cluster
	return &i
}

// WithReplication is a no-op for this operation.
func (i InsertIntoTable) WithReplication() Operation {
	return &i
}

// ShouldWaitForDistributionQueue returns false as INSERTs do not start DDL queues.
func (i InsertIntoTable) ShouldWaitForDistributionQueue() (bool, string, string) {
	return false, i.Database, i.Table
}

// IsMutation returns false (does not alter existing data parts).
func (i InsertIntoTable) IsMutation() bool { return false }

// IsIdempotent returns false (re-inserting duplicates data).
func (i InsertIntoTable) IsIdempotent() bool { return false }

// IsLightweight returns true.
func (i InsertIntoTable) IsLightweight() bool { return true }

func (i InsertIntoTable) ToSQL() string {
	var sql strings.Builder
	sql.WriteString("INSERT INTO ")
	sql.WriteString(i.Database)
	sql.WriteString(".")
	sql.WriteString(i.Table)

	if len(i.Columns) > 0 {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(i.Columns, ", "))
		sql.WriteString(")")
	}

	sql.WriteString(" VALUES ")

	rows := make([]string, 0, len(i.Values))
	for _, row := range i.Values {
		vals := make([]string, len(row))
		for idx, v := range row {
			vals[idx] = encodeSQLValue(v)
		}
		rows = append(rows, fmt.Sprintf("(%s)", strings.Join(vals, ", ")))
	}

	sql.WriteString(strings.Join(rows, ", "))
	return sql.String()
}

// encodeSQLValue converts a Go value into a ClickHouse SQL literal.
// Keep intentionally small and safe.
func encodeSQLValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "NULL"
	case string:
		return quoteString(val)
	case []byte:
		// treat as string payload
		return quoteString(string(val))
	case bool:
		if val {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(val)
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32:
		f := float64(val)
		if !utils.IsValidFloat(f) {
			return "NULL"
		}
		return trimFloatZeros(strconv.FormatFloat(f, 'f', -1, 64))
	case float64:
		if !utils.IsValidFloat(val) {
			return "NULL"
		}
		return trimFloatZeros(strconv.FormatFloat(val, 'f', -1, 64))
	default:
		// Fallback to string representation
		return quoteString(fmt.Sprint(val))
	}
}

func quoteString(s string) string {
	// Escape backslashes first, then single quotes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return "'" + s + "'"
}

func trimFloatZeros(s string) string {
	// Keep as-is for simplicity; ClickHouse accepts plain decimal strings
	return s
}
