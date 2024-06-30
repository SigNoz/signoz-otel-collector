CREATE TABLE IF NOT EXISTS signoz_traces.signoz_index ON CLUSTER {{.SIGNOZ_CLUSTER}} (
  timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
  traceID String CODEC(ZSTD(1)),
  spanID String CODEC(ZSTD(1)),
  parentSpanID String CODEC(ZSTD(1)),
  serviceName LowCardinality(String) CODEC(ZSTD(1)),
  name LowCardinality(String) CODEC(ZSTD(1)),
  kind Int32 CODEC(ZSTD(1)),
  durationNano UInt64 CODEC(ZSTD(1)),
  tags Array(String) CODEC(ZSTD(1)),
  tagsKeys Array(String) CODEC(ZSTD(1)),
  tagsValues Array(String) CODEC(ZSTD(1)),
  statusCode Int64 CODEC(ZSTD(1)),
  references String CODEC(ZSTD(1)),
  externalHttpMethod Nullable(String) CODEC(ZSTD(1)),
  externalHttpUrl Nullable(String) CODEC(ZSTD(1)),
  component Nullable(String) CODEC(ZSTD(1)),
  dbSystem Nullable(String) CODEC(ZSTD(1)),
  dbName Nullable(String) CODEC(ZSTD(1)),
  dbOperation Nullable(String) CODEC(ZSTD(1)),
  peerService Nullable(String) CODEC(ZSTD(1)),
  INDEX idx_traceID traceID TYPE bloom_filter GRANULARITY 4,
  INDEX idx_service serviceName TYPE bloom_filter GRANULARITY 4,
  INDEX idx_name name TYPE bloom_filter GRANULARITY 4,
  INDEX idx_kind kind TYPE minmax GRANULARITY 4,
  INDEX idx_tagsKeys tagsKeys TYPE bloom_filter(0.01) GRANULARITY 64,
  INDEX idx_tagsValues tagsValues TYPE bloom_filter(0.01) GRANULARITY 64,
  INDEX idx_duration durationNano TYPE minmax GRANULARITY 1
) ENGINE {{.SIGNOZ_REPLICATED}}MergeTree
PARTITION BY toDate(timestamp)
ORDER BY (serviceName, -toUnixTimestamp(timestamp));
