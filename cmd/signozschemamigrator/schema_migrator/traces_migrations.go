package schemamigrator

var TracesMigrations = []SchemaMigrationRecord{
	{
		MigrationID: 20240818,
		UpItems: []Operation{
			AlterTableDropIndex{
				Database: "signoz_traces",
				Table:    "signoz_index_v2",
				Index:    Index{Name: "idx_resourceTagsMapKeys"},
			},
			AlterTableDropIndex{
				Database: "signoz_traces",
				Table:    "signoz_index_v2",
				Index:    Index{Name: "idx_resourceTagsMapValues"},
			},
			AlterTableAddIndex{
				Database: "signoz_traces",
				Table:    "signoz_index_v2",
				Index:    Index{Name: "idx_resourceTagMapKeys_tokenbf", Expression: "mapKeys(resourceTagsMap)", Type: "tokenbf_v1(1024, 2, 0)", Granularity: 1},
			},
			AlterTableAddIndex{
				Database: "signoz_traces",
				Table:    "signoz_index_v2",
				Index:    Index{Name: "idx_resourceTagMapValues_ngram", Expression: "mapValues(resourceTagsMap)", Type: "ngrambf_v1(4, 5000, 2, 0)", Granularity: 1},
			},
		},
	},
}
