DROP TABLE IF EXISTS signoz_traces.durationSortMV ON CLUSTER {{.SIGNOZ_CLUSTER}};

ALTER TABLE signoz_traces.signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_signoz_index_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}} 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}} 
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));

ALTER TABLE signoz_traces.distributed_durationSort ON CLUSTER {{.SIGNOZ_CLUSTER}}
DROP COLUMN IF NOT EXISTS stringTagMap Map(String, String) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS numberTagMap Map(String, Float64) CODEC(ZSTD(1)),
DROP COLUMN IF NOT EXISTS boolTagMap Map(String, bool) CODEC(ZSTD(1));
