package constants

const (
	BodyJSONColumn           = "body_json"
	BodyJSONColumnPrefix     = "body_json."
	BodyPromotedColumn       = "body_json_promoted"
	BodyPromotedColumnPrefix = "body_json_promoted."

	SignozMetadataDB              = "signoz_metadata"
	LocalPathTypesTable           = "json_path_types"
	DistributedPathTypesTable     = "distributed_json_path_types"
	LocalPromotedPathsTable       = "json_promoted_paths"
	DistributedPromotedPathsTable = "distributed_json_promoted_paths"
	PathTypesTablePathColumn      = "path"
	PathTypesTableTypeColumn      = "type"
	PathTypesTableLastSeenColumn  = "last_seen"
)
