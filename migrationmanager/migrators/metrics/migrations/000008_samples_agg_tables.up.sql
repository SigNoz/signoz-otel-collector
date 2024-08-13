-- 5m aggregation table
CREATE TABLE IF NOT EXISTS signoz_metrics.samples_v4_agg_5m ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `env` LowCardinality(String) DEFAULT 'default',
    `temporality` LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name` LowCardinality(String),
    `fingerprint` UInt64 CODEC(ZSTD(1)),
    `unix_milli` Int64 CODEC(DoubleDelta, ZSTD(1)),
    `last` SimpleAggregateFunction(anyLast, Float64) CODEC(ZSTD(1)),
    `min` SimpleAggregateFunction(min, Float64) CODEC(ZSTD(1)),
    `max` SimpleAggregateFunction(max, Float64) CODEC(ZSTD(1)),
    `sum` SimpleAggregateFunction(sum, Float64) CODEC(ZSTD(1)),
    `count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = {{.SIGNOZ_REPLICATED}}AggregatingMergeTree
    PARTITION BY toDate(unix_milli / 1000)
    ORDER BY (env, temporality, metric_name, fingerprint, unix_milli)
    TTL toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE
    SETTINGS ttl_only_drop_parts = 1;

-- 30m aggregation table
CREATE TABLE IF NOT EXISTS signoz_metrics.samples_v4_agg_30m ON CLUSTER {{.SIGNOZ_CLUSTER}}
(
    `env` LowCardinality(String) DEFAULT 'default',
    `temporality` LowCardinality(String) DEFAULT 'Unspecified',
    `metric_name` LowCardinality(String),
    `fingerprint` UInt64 CODEC(ZSTD(1)),
    `unix_milli` Int64 CODEC(DoubleDelta, ZSTD(1)),
    `last` SimpleAggregateFunction(anyLast, Float64) CODEC(ZSTD(1)),
    `min` SimpleAggregateFunction(min, Float64) CODEC(ZSTD(1)),
    `max` SimpleAggregateFunction(max, Float64) CODEC(ZSTD(1)),
    `sum` SimpleAggregateFunction(sum, Float64) CODEC(ZSTD(1)),
    `count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = {{.SIGNOZ_REPLICATED}}AggregatingMergeTree
    PARTITION BY toDate(unix_milli / 1000)
    ORDER BY (env, temporality, metric_name, fingerprint, unix_milli)
    TTL toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE
    SETTINGS ttl_only_drop_parts = 1;

-- 5m aggregation materialized view
CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_metrics.samples_v4_agg_5m_mv ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_metrics.samples_v4_agg_5m AS
SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 300000) * 300000 as unix_milli,
    anyLast(value) as last,
    min(value) as min,
    max(value) as max,
    sum(value) as sum,
    count(*) as count
FROM signoz_metrics.samples_v4 
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;

-- 30m aggregation materialized view
CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_metrics.samples_v4_agg_30m_mv ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_metrics.samples_v4_agg_30m AS
SELECT
    env,
    temporality,
    metric_name,
    fingerprint,
    intDiv(unix_milli, 1800000) * 1800000 as unix_milli,
    anyLast(last) as last,
    min(min) as min,
    max(max) as max,
    sum(sum) as sum,
    sum(count) as count
FROM signoz_metrics.samples_v4_agg_5m 
GROUP BY
    env,
    temporality,
    metric_name,
    fingerprint,
    unix_milli;

-- 5m aggregation distributed table
CREATE TABLE IF NOT EXISTS signoz_metrics.distributed_samples_v4_agg_5m ON CLUSTER {{.SIGNOZ_CLUSTER}}
AS signoz_metrics.samples_v4_agg_5m
ENGINE = Distributed('{{.SIGNOZ_CLUSTER}}', 'signoz_metrics', 'samples_v4_agg_5m', cityHash64(env, temporality, metric_name, fingerprint));

-- 30m aggregation distributed table
CREATE TABLE IF NOT EXISTS signoz_metrics.distributed_samples_v4_agg_30m ON CLUSTER {{.SIGNOZ_CLUSTER}}
AS signoz_metrics.samples_v4_agg_30m
ENGINE = Distributed('{{.SIGNOZ_CLUSTER}}', 'signoz_metrics', 'samples_v4_agg_30m', cityHash64(env, temporality, metric_name, fingerprint));

-- time series v4 1 week table
CREATE TABLE IF NOT EXISTS signoz_metrics.time_series_v4_1week ON CLUSTER {{.SIGNOZ_CLUSTER}} (
    env LowCardinality(String) DEFAULT 'default',
    temporality LowCardinality(String) DEFAULT 'Unspecified',
    metric_name LowCardinality(String),
    description LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    unit LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    type LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    is_monotonic Bool DEFAULT false CODEC(ZSTD(1)),
    fingerprint UInt64 CODEC(Delta, ZSTD),
    unix_milli Int64 CODEC(Delta, ZSTD),
    labels String CODEC(ZSTD(5)),
    INDEX idx_labels labels TYPE ngrambf_v1(4, 1024, 3, 0) GRANULARITY 1
)
ENGINE = {{.SIGNOZ_REPLICATED}}ReplacingMergeTree
        PARTITION BY toDate(unix_milli / 1000)
        ORDER BY (env, temporality, metric_name, fingerprint, unix_milli)
        TTL toDateTime(unix_milli/1000) + INTERVAL 2592000 SECOND DELETE
        SETTINGS ttl_only_drop_parts = 1;

-- time series v4 1 week materialized view
CREATE MATERIALIZED VIEW IF NOT EXISTS signoz_metrics.time_series_v4_1week_mv ON CLUSTER {{.SIGNOZ_CLUSTER}}
TO signoz_metrics.time_series_v4_1week
AS SELECT
    env,
    temporality,
    metric_name,
    description,
    unit,
    type,
    is_monotonic,
    fingerprint,
    floor(unix_milli/604800000)*604800000 AS unix_milli,
    labels
FROM signoz_metrics.time_series_v4_1day;

-- time series v4 1 week distributed table
CREATE TABLE IF NOT EXISTS signoz_metrics.distributed_time_series_v4_1week ON CLUSTER {{.SIGNOZ_CLUSTER}}
AS signoz_metrics.time_series_v4_1week
ENGINE = Distributed("{{.SIGNOZ_CLUSTER}}", signoz_metrics, time_series_v4_1week, cityHash64(env, temporality, metric_name, fingerprint));
