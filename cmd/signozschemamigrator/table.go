package main

import (
	"fmt"
	"strings"
)

type TableEngine interface {
	ToSQL() string
}

type TableSetting struct {
	Name  string
	Value string
}

func (t *TableSetting) ToSQL() string {
	return fmt.Sprintf("%s = %s", t.Name, t.Value)
}

type TableSettings []TableSetting

func (t *TableSettings) ToSQL() string {
	parts := make([]string, len(*t))
	for i, setting := range *t {
		parts[i] = setting.ToSQL()
	}
	return strings.Join(parts, ", ")
}

type MergeTree struct {
	Replicated  bool
	OrderBy     string
	PrimaryKey  string
	PartitionBy string
	SampleBy    string
	TTL         string
	Settings    TableSettings
}

func (m *MergeTree) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(m.EngineType())
	if m.PrimaryKey != "" {
		sql.WriteString(" PRIMARY KEY ")
		sql.WriteString(m.PrimaryKey)
	}
	sql.WriteString(" ORDER BY ")
	sql.WriteString(m.OrderBy)
	if m.PartitionBy != "" {
		sql.WriteString(" PARTITION BY ")
		sql.WriteString(m.PartitionBy)
	}
	if m.SampleBy != "" {
		sql.WriteString(" SAMPLE BY ")
		sql.WriteString(m.SampleBy)
	}
	if m.TTL != "" {
		sql.WriteString(" TTL ")
		sql.WriteString(m.TTL)
	}
	if len(m.Settings) > 0 {
		sql.WriteString(" SETTINGS ")
		sql.WriteString(m.Settings.ToSQL())
	}
	return sql.String()
}

func (m *MergeTree) EngineType() string {
	if m.Replicated {
		return "ReplicatedMergeTree"
	}
	return "MergeTree"
}

type ReplicatedMergeTree struct {
	MergeTree
}

func (r *ReplicatedMergeTree) EngineType() string {
	if r.Replicated {
		return "ReplicatedReplacingMergeTree"
	}
	return "ReplacingMergeTree"
}

type AggregatingMergeTree struct {
	MergeTree
}

func (a *AggregatingMergeTree) EngineType() string {
	if a.Replicated {
		return "ReplicatedAggregatingMergeTree"
	}
	return "AggregatingMergeTree"
}

type Distributed struct {
	Cluster     string
	Database    string
	Table       string
	ShardingKey string
}

func (d *Distributed) ToSQL() string {
	return fmt.Sprintf("Distributed('%s', %s, %s, %s)", d.Cluster, d.Database, d.Table, d.ShardingKey)
}
