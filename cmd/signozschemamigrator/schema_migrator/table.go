package schemamigrator

import (
	"fmt"
	"strings"
)

// TableEngine represents the engine of the table.
type TableEngine interface {
	ToSQL() string
	EngineType() string
	OnCluster(cluster string) TableEngine
	WithReplication() TableEngine
}

// TableSetting represents the setting of the table.
type TableSetting struct {
	Name  string
	Value string
}

func (t TableSetting) ToSQL() string {
	return fmt.Sprintf("%s = %s", t.Name, t.Value)
}

type TableSettings []TableSetting

func (t TableSettings) ToSQL() string {
	parts := make([]string, len(t))
	for i, setting := range t {
		parts[i] = setting.ToSQL()
	}
	return strings.Join(parts, ", ")
}

// MergeTree represents the MergeTree engine of the table.
type MergeTree struct {
	Replicated  bool
	OrderBy     string
	PrimaryKey  string
	PartitionBy string
	SampleBy    string
	TTL         string
	Settings    TableSettings
}

func (m MergeTree) OnCluster(cluster string) TableEngine {
	// no-op
	return &m
}

func (m MergeTree) WithReplication() TableEngine {
	m.Replicated = true
	return &m
}

func (m MergeTree) EngineParams() string {
	var sql strings.Builder
	if m.PrimaryKey != "" {
		sql.WriteString(" PRIMARY KEY ")
		sql.WriteString(m.PrimaryKey)
	}
	if m.OrderBy != "" {
		sql.WriteString(" ORDER BY ")
		sql.WriteString(m.OrderBy)
	}
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

func (m MergeTree) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(m.EngineType())
	sql.WriteString(m.EngineParams())
	return sql.String()
}

func (m MergeTree) EngineType() string {
	if m.Replicated {
		return "ReplicatedMergeTree"
	}
	return "MergeTree"
}

// Replacing represents the ReplacingMergeTree engine of the table.
type ReplacingMergeTree struct {
	MergeTree
}

func (r ReplacingMergeTree) OnCluster(cluster string) TableEngine {
	return &r
}

func (r ReplacingMergeTree) WithReplication() TableEngine {
	r.Replicated = true
	return &r
}

func (r ReplacingMergeTree) EngineType() string {
	if r.Replicated {
		return "ReplicatedReplacingMergeTree"
	}
	return "ReplacingMergeTree"
}

func (r ReplacingMergeTree) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(r.EngineType())
	sql.WriteString(r.EngineParams())
	return sql.String()
}

// AggregatingMergeTree represents the AggregatingMergeTree engine of the table.
type AggregatingMergeTree struct {
	MergeTree
}

func (a AggregatingMergeTree) OnCluster(cluster string) TableEngine {
	return &a
}

func (a AggregatingMergeTree) WithReplication() TableEngine {
	a.Replicated = true
	return &a
}

func (a AggregatingMergeTree) EngineType() string {
	if a.Replicated {
		return "ReplicatedAggregatingMergeTree"
	}
	return "AggregatingMergeTree"
}

func (a AggregatingMergeTree) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(a.EngineType())
	sql.WriteString(a.EngineParams())
	return sql.String()
}

type SummingMergeTree struct {
	MergeTree
}

func (s SummingMergeTree) OnCluster(cluster string) TableEngine {
	return &s
}

func (s SummingMergeTree) WithReplication() TableEngine {
	s.Replicated = true
	return &s
}

func (s SummingMergeTree) EngineType() string {
	if s.Replicated {
		return "ReplicatedSummingMergeTree"
	}
	return "SummingMergeTree"
}

func (s SummingMergeTree) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(s.EngineType())
	sql.WriteString(s.EngineParams())
	return sql.String()
}

// Distributed represents the Distributed engine of the table.
type Distributed struct {
	Cluster     string
	Database    string
	Table       string
	ShardingKey string
}

func (d Distributed) OnCluster(cluster string) TableEngine {
	d.Cluster = cluster
	return &d
}

func (d Distributed) WithReplication() TableEngine {
	// no-op
	return &d
}

func (d Distributed) EngineType() string {
	return "Distributed"
}

func (d Distributed) EngineParams() string {
	return ""
}

func (d Distributed) ToSQL() string {
	return fmt.Sprintf("Distributed('%s', %s, %s, %s)", d.Cluster, d.Database, d.Table, d.ShardingKey)
}
