package schemamigrator

var TracesMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 1000,
		UpItems: []Operation{
			AlterTableDropIndex{Database: "signoz_traces", Table: "signoz_index_v2", Index: Index{Name: "idx_resourceTagsMapKeys"}},
		},
		DownItems: []Operation{
			AlterTableAddIndex{Database: "signoz_traces", Table: "signoz_index_v2", Index: Index{Name: "idx_resourceTagsMapKeys", Expression: "mapKeys(resourceTagsMap)", Type: "bloom_filter(0.01)", Granularity: 64}},
		},
	},
}
