package constants

const (
	BodyV2Column             = "body_v2"
	BodyV2ColumnPrefix       = "body_v2."
	BodyPromotedColumn       = "body_promoted"
	BodyPromotedColumnPrefix = "body_promoted."

	SignozMetadataDB             = "signoz_metadata"
	DistTableColumnEvolution     = SignozMetadataDB + ".distributed_column_evolution_metadata"
	LocalFieldKeysTable          = "field_keys"
	DistributedFieldKeysTable    = "distributed_field_keys"
	FieldKeysTableNameColumn     = "field_name"
	FieldKeysTableDataTypeColumn = "field_data_type"
	FieldKeysTableLastSeenColumn = "last_seen"

	TracesColumnAttributesPromoted = "attributes_promoted"
)
