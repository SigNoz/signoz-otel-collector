package main

import (
	"fmt"
	"strings"
)

type ColumnProperty string

const (
	ColumnPropertyTTL          ColumnProperty = "TTL"
	ColumnPropertyDefault      ColumnProperty = "DEFAULT"
	ColumnPropertyAlias        ColumnProperty = "ALIAS"
	ColumnPropertyMaterialized ColumnProperty = "MATERIALIZED"
	ColumnPropertyCodec        ColumnProperty = "CODEC"
	ColumnPropertyComment      ColumnProperty = "COMMENT"
	ColumnPropertySettings     ColumnProperty = "SETTINGS"
)

type ColumnType interface {
	String() string
}

type PrimitiveColumnType string

const (
	// String types
	ColumnTypeString      PrimitiveColumnType = "String"
	ColumnTypeFixedString PrimitiveColumnType = "FixedString"
	// Integer types
	ColumnTypeInt8    PrimitiveColumnType = "Int8"
	ColumnTypeInt16   PrimitiveColumnType = "Int16"
	ColumnTypeInt32   PrimitiveColumnType = "Int32"
	ColumnTypeInt64   PrimitiveColumnType = "Int64"
	ColumnTypeInt128  PrimitiveColumnType = "Int128"
	ColumnTypeInt256  PrimitiveColumnType = "Int256"
	ColumnTypeUInt8   PrimitiveColumnType = "UInt8"
	ColumnTypeUInt16  PrimitiveColumnType = "UInt16"
	ColumnTypeUInt32  PrimitiveColumnType = "UInt32"
	ColumnTypeUInt64  PrimitiveColumnType = "UInt64"
	ColumnTypeUInt128 PrimitiveColumnType = "UInt128"
	ColumnTypeUInt256 PrimitiveColumnType = "UInt256"
	// Floating point types
	ColumnTypeFloat32 PrimitiveColumnType = "Float32"
	ColumnTypeFloat64 PrimitiveColumnType = "Float64"
	// Boolean types
	ColumnTypeBool PrimitiveColumnType = "Bool"
	// Date types
	ColumnTypeDate       PrimitiveColumnType = "Date"
	ColumnTypeDate32     PrimitiveColumnType = "Date32"
	ColumnTypeDateTime   PrimitiveColumnType = "DateTime"
	ColumnTypeDateTime64 PrimitiveColumnType = "DateTime64"
	// UUID types
	ColumnTypeUUID PrimitiveColumnType = "UUID"
	// IP types
	ColumnTypeIPv4 PrimitiveColumnType = "IPv4"
	ColumnTypeIPv6 PrimitiveColumnType = "IPv6"
)

func (p PrimitiveColumnType) String() string {
	return string(p)
}

type ArrayColumnType struct {
	ElementType ColumnType
}

func (a ArrayColumnType) String() string {
	return fmt.Sprintf("Array(%s)", a.ElementType)
}

type MapColumnType struct {
	KeyType   ColumnType
	ValueType ColumnType
}

func (m MapColumnType) String() string {
	return fmt.Sprintf("Map(%s, %s)", m.KeyType, m.ValueType)
}

type TupleColumnType struct {
	ElementTypes []ColumnType
}

func (t TupleColumnType) String() string {
	elements := make([]string, len(t.ElementTypes))
	for i, et := range t.ElementTypes {
		elements[i] = et.String()
	}
	return fmt.Sprintf("Tuple(%s)", strings.Join(elements, ", "))
}

type LowCardinalityColumnType struct {
	ElementType ColumnType
}

func (l LowCardinalityColumnType) String() string {
	return fmt.Sprintf("LowCardinality(%s)", l.ElementType)
}

type NullableColumnType struct {
	ElementType ColumnType
}

func (n NullableColumnType) String() string {
	return fmt.Sprintf("Nullable(%s)", n.ElementType)
}

type SimpleAggregateFunction struct {
	FunctionName string
	Arguments    []ColumnType
}

func (s SimpleAggregateFunction) String() string {
	arguments := make([]string, len(s.Arguments))
	for i, arg := range s.Arguments {
		arguments[i] = arg.String()
	}
	return fmt.Sprintf("SimpleAggregateFunction(%s, %s)", s.FunctionName, strings.Join(arguments, ", "))
}

type AggregateFunction struct {
	FunctionName string
	Arguments    []ColumnType
}

func (a AggregateFunction) String() string {
	arguments := make([]string, len(a.Arguments))
	for i, arg := range a.Arguments {
		arguments[i] = arg.String()
	}
	return fmt.Sprintf("AggregateFunction(%s, %s)", a.FunctionName, strings.Join(arguments, ", "))
}

type Column struct {
	Name     string
	Type     ColumnType
	Codec    string
	Default  string
	TTL      string
	Settings ColumnSettings
	Comment  string
}

func (c Column) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(c.Name)
	sql.WriteString(" ")
	sql.WriteString(c.Type.String())
	if c.Default != "" {
		sql.WriteString(" DEFAULT ")
		sql.WriteString(c.Default)
	}
	if c.Codec != "" {
		sql.WriteString(" CODEC ")
		sql.WriteString(c.Codec)
	}
	if c.TTL != "" {
		sql.WriteString(" TTL ")
		sql.WriteString(c.TTL)
	}
	if c.Settings != nil {
		sql.WriteString(" SETTINGS ")
		sql.WriteString(c.Settings.String())
	}
	if c.Comment != "" {
		sql.WriteString(" COMMENT ")
		sql.WriteString(c.Comment)
	}
	return sql.String()
}

type ColumnSetting struct {
	Name  string
	Value string
}

func (c ColumnSetting) String() string {
	return fmt.Sprintf("%s = %s", c.Name, c.Value)
}

type ColumnSettings []ColumnSetting

func (c ColumnSettings) Names() []string {
	names := make([]string, len(c))
	for i, s := range c {
		names[i] = s.Name
	}
	return names
}

func (c ColumnSettings) String() string {
	settings := make([]string, len(c))
	for i, s := range c {
		settings[i] = s.String()
	}
	return strings.Join(settings, ", ")
}
