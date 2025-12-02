package schemamigrator

import (
	"fmt"
	"strings"
)

// ColumnProperty represents a column property.
// It is used to represent the column property in the column definition.
// Example: TTL, DEFAULT, ALIAS, etc.
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

type ColumnTypeEnum int

const (
	// String types
	ColumnTypeEnumString ColumnTypeEnum = iota
	// Integer types
	ColumnTypeEnumInt8
	ColumnTypeEnumInt16
	ColumnTypeEnumInt32
	ColumnTypeEnumInt64
	ColumnTypeEnumInt128
	ColumnTypeEnumInt256
	ColumnTypeEnumUInt8
	ColumnTypeEnumUInt16
	ColumnTypeEnumUInt32
	ColumnTypeEnumUInt64
	ColumnTypeEnumUInt128
	ColumnTypeEnumUInt256
	// Floating point types
	ColumnTypeEnumFloat32
	ColumnTypeEnumFloat64
	// Boolean types
	ColumnTypeEnumBool
	// Date types
	ColumnTypeEnumDate
	ColumnTypeEnumDate32
	// UUID types
	ColumnTypeEnumUUID
	// IP types
	ColumnTypeEnumIPv4
	ColumnTypeEnumIPv6

	ColumnTypeEnumLowCardinality
	ColumnTypeEnumNullable
	ColumnTypeEnumSimpleAggregateFunction
	ColumnTypeEnumAggregateFunction
	ColumnTypeEnumJSON
	ColumnTypeEnumFixedString
	ColumnTypeEnumDateTime
	ColumnTypeEnumDateTime64
	ColumnTypeEnumArray
	ColumnTypeEnumMap
	ColumnTypeEnumTuple
	ColumnTypeEnumEnumeration
)

// ColumnType represents a column type.
// It is used to represent the column type in the schema definition.
// The column type can be a primitive type, a fixed string type, a date time type,
// an array type, a map type, a tuple type, a low cardinality type, or a nullable type.
// a simple aggregate function type, or an aggregate function type.
type ColumnType interface {
	String() string
	GetType() ColumnTypeEnum
}

// PrimitiveColumnType represents a primitive column type.
// It is used to represent the primitive column type in the column type.
// Example: String, Int64, Float64, Bool, Date, DateTime, etc.
type PrimitiveColumnType struct {
	Type string
	Enum ColumnTypeEnum
}

func (p PrimitiveColumnType) String() string {
	return p.Type
}

func (p PrimitiveColumnType) GetType() ColumnTypeEnum {
	return p.Enum
}

var (
	// String types
	ColumnTypeString = PrimitiveColumnType{Type: "String", Enum: ColumnTypeEnumString}
	// Integer types
	ColumnTypeInt8    = PrimitiveColumnType{Type: "Int8", Enum: ColumnTypeEnumInt8}
	ColumnTypeInt16   = PrimitiveColumnType{Type: "Int16", Enum: ColumnTypeEnumInt16}
	ColumnTypeInt32   = PrimitiveColumnType{Type: "Int32", Enum: ColumnTypeEnumInt32}
	ColumnTypeInt64   = PrimitiveColumnType{Type: "Int64", Enum: ColumnTypeEnumInt64}
	ColumnTypeInt128  = PrimitiveColumnType{Type: "Int128", Enum: ColumnTypeEnumInt128}
	ColumnTypeInt256  = PrimitiveColumnType{Type: "Int256", Enum: ColumnTypeEnumInt256}
	ColumnTypeUInt8   = PrimitiveColumnType{Type: "UInt8", Enum: ColumnTypeEnumUInt8}
	ColumnTypeUInt16  = PrimitiveColumnType{Type: "UInt16", Enum: ColumnTypeEnumUInt16}
	ColumnTypeUInt32  = PrimitiveColumnType{Type: "UInt32", Enum: ColumnTypeEnumUInt32}
	ColumnTypeUInt64  = PrimitiveColumnType{Type: "UInt64", Enum: ColumnTypeEnumUInt64}
	ColumnTypeUInt128 = PrimitiveColumnType{Type: "UInt128", Enum: ColumnTypeEnumUInt128}
	ColumnTypeUInt256 = PrimitiveColumnType{Type: "UInt256", Enum: ColumnTypeEnumUInt256}

	// Floating point types
	ColumnTypeFloat32 = PrimitiveColumnType{Type: "Float32", Enum: ColumnTypeEnumFloat32}
	ColumnTypeFloat64 = PrimitiveColumnType{Type: "Float64", Enum: ColumnTypeEnumFloat64}
	// Boolean types
	ColumnTypeBool = PrimitiveColumnType{Type: "Bool", Enum: ColumnTypeEnumBool}
	// Date types
	ColumnTypeDate   = PrimitiveColumnType{Type: "Date", Enum: ColumnTypeEnumDate}
	ColumnTypeDate32 = PrimitiveColumnType{Type: "Date32", Enum: ColumnTypeEnumDate32}
	// UUID types
	ColumnTypeUUID = PrimitiveColumnType{Type: "UUID", Enum: ColumnTypeEnumUUID}
	// IP types
	ColumnTypeIPv4 = PrimitiveColumnType{Type: "IPv4", Enum: ColumnTypeEnumIPv4}
	ColumnTypeIPv6 = PrimitiveColumnType{Type: "IPv6", Enum: ColumnTypeEnumIPv6}
)

// JSONColumnType represent a JSON column type
type JSONColumnType struct {
	MaxDynamicPaths *uint
	MaxDynamicTypes *uint
	Columns         []Column
}

func (f JSONColumnType) String() string {
	params := []string{}
	if f.MaxDynamicPaths != nil {
		params = append(params, fmt.Sprintf("max_dynamic_paths=%d", *f.MaxDynamicPaths))
	}
	if f.MaxDynamicTypes != nil {
		params = append(params, fmt.Sprintf("max_dynamic_types=%d", *f.MaxDynamicTypes))
	}

	for _, column := range f.Columns {
		params = append(params, fmt.Sprintf("%s %s", column.Name, column.Type.String()))
	}

	return fmt.Sprintf("JSON(%s)", strings.Join(params, ", "))
}

func (f JSONColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumJSON
}

// FixedStringColumnType represents a fixed string column type.
// It is used to represent the fixed string column type in the column type.
// Length is the length of the fixed string column type.
// Example: FixedString(256), where 256 is the length.
type FixedStringColumnType struct {
	Length int
}

func (f FixedStringColumnType) String() string {
	return fmt.Sprintf("FixedString(%d)", f.Length)
}

func (f FixedStringColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumFixedString
}

// DateTimeColumnType represents a date time column type.
// It is used to represent the date time column type in the column type.
// Timezone is the timezone of the date time column type.
// Example: DateTime('UTC'), where UTC is the timezone.
type DateTimeColumnType struct {
	Timezone string
}

func (d DateTimeColumnType) String() string {
	if d.Timezone == "" {
		return "DateTime"
	}
	return fmt.Sprintf("DateTime(%s)", d.Timezone)
}

func (d DateTimeColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumDateTime
}

// DateTime64ColumnType represents a date time 64 column type.
// It is used to represent the date time 64 column type in the column type.
// Precision is the precision of the date time 64 column type.
// Timezone is the timezone of the date time 64 column type.
type DateTime64ColumnType struct {
	Precision int
	Timezone  string
}

func (d DateTime64ColumnType) String() string {
	if d.Timezone == "" {
		return fmt.Sprintf("DateTime64(%d)", d.Precision)
	}
	return fmt.Sprintf("DateTime64(%d, %s)", d.Precision, d.Timezone)
}

func (d DateTime64ColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumDateTime64
}

// ArrayColumnType represents an array column type.
// It is used to represent the array column type in the column type.
// ElementType is the element type of the array column type.
// Example: Array(Int64), where Int64 is the element type.
type ArrayColumnType struct {
	ElementType ColumnType
}

func (a ArrayColumnType) String() string {
	return fmt.Sprintf("Array(%s)", a.ElementType)
}

func (a ArrayColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumArray
}

// MapColumnType represents a map column type.
// It is used to represent the map column type in the column type.
// KeyType is the key type of the map column type.
// ValueType is the value type of the map column type.
// Example: Map(String, Int64), where String is the key type and Int64 is the value type.
// The key/value type can be any arbitrary type.
type MapColumnType struct {
	KeyType   ColumnType
	ValueType ColumnType
}

func (m MapColumnType) String() string {
	return fmt.Sprintf("Map(%s, %s)", m.KeyType, m.ValueType)
}

func (m MapColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumMap
}

// TupleColumnType represents a tuple column type.
// It is used to represent the tuple column type in the column type.
// ElementTypes is the element types of the tuple column type.
// Example: Tuple(Int64, String), where Int64 and String are the element types.
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

func (t TupleColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumTuple
}

// LowCardinalityColumnType represents a low cardinality column type.
// It is used to represent the low cardinality column type in the column type.
// ElementType is the element type of the low cardinality column type.
// Example: LowCardinality(String), where String is the element type.
type LowCardinalityColumnType struct {
	ElementType ColumnType
}

func (l LowCardinalityColumnType) String() string {
	return fmt.Sprintf("LowCardinality(%s)", l.ElementType)
}

func (l LowCardinalityColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumLowCardinality
}

type EnumerationColumnType struct {
	Values []string
	Size   int
}

func (e EnumerationColumnType) String() string {
	return fmt.Sprintf("Enum%d(%s)", e.Size, strings.Join(e.Values, ", "))
}

func (e EnumerationColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumEnumeration
}

// NullableColumnType represents a nullable column type.
// It is used to represent the nullable column type in the column type.
// ElementType is the element type of the nullable column type.
// Example: Nullable(String), where String is the element type.
type NullableColumnType struct {
	ElementType ColumnType
}

func (n NullableColumnType) String() string {
	return fmt.Sprintf("Nullable(%s)", n.ElementType)
}

func (n NullableColumnType) GetType() ColumnTypeEnum {
	return ColumnTypeEnumNullable
}

// SimpleAggregateFunction represents a simple aggregate function in a column type.
// It is used to represent the simple aggregate function in the column type.
// FunctionName is the name of the simple aggregate function.
// Arguments are the arguments of the simple aggregate function.
// Example: SimpleAggregateFunction(sum, Int64), where sum is the function name
// and Int64 is the argument.
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

func (s SimpleAggregateFunction) GetType() ColumnTypeEnum {
	return ColumnTypeEnumSimpleAggregateFunction
}

// AggregateFunction represents an aggregate function in a column type.
// It is used to represent the aggregate function in the column type.
// FunctionName is the name of the aggregate function.
// Arguments are the arguments of the aggregate function.
// Example: AggregateFunction(sum, Int64), where sum is the function name
// and Int64 is the argument.
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

func (a AggregateFunction) GetType() ColumnTypeEnum {
	return ColumnTypeEnumAggregateFunction
}

// Column represents a column in a table.
// It is used to represent the column in the schema definition.
// Name (mandatory) is the name of the column.
// Type (mandatory) is the type of the column.
// Codec is the codec of the column.
// Alias is the name of the column we want to reference in the schema definition.
// Default is the default value/expression of the column. It is
// user responsibility to ensure that the default value is valid
// for the column type. The migrator will not validate the default
// value for you.
// TTL is the ttl of the column.
// Settings is the settings of the column.
// Comment is the comment of the column.
type Column struct {
	Name         string
	Type         ColumnType
	Codec        string
	Alias        string
	Default      string
	Materialized string
	TTL          string
	Settings     ColumnSettings
	Comment      string
}

func (c Column) ToSQL() string {
	var sql strings.Builder
	sql.WriteString(c.Name)
	sql.WriteString(" ")
	sql.WriteString(c.Type.String())
	if c.Alias != "" {
		sql.WriteString(" ALIAS ")
		sql.WriteString(c.Alias)
	}
	if c.Default != "" {
		sql.WriteString(" DEFAULT ")
		sql.WriteString(c.Default)
	}
	if c.Materialized != "" {
		sql.WriteString(" MATERIALIZED ")
		sql.WriteString(c.Materialized)
	}
	if c.Codec != "" {
		sql.WriteString(" CODEC(")
		sql.WriteString(c.Codec)
		sql.WriteString(")")
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
