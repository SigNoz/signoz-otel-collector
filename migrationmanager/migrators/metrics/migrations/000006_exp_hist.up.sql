CREATE TABLE IF NOT EXISTS signoz_metrics.exp_hist ON CLUSTER {{.SIGNOZ_CLUSTER}} (
                        metric_name LowCardinality(String),
                        fingerprint UInt64 Codec(DoubleDelta, LZ4),
                        sketch AggregateFunction(quantilesDDSketch(0.01, 0.9), UInt64),
                        timestamp_ms Int64 Codec(DoubleDelta, LZ4)
                )
                ENGINE = MergeTree
                        PARTITION BY toDate(timestamp_ms / 1000)
                        ORDER BY (metric_name, fingerprint, timestamp_ms);

CREATE TABLE IF NOT EXISTS signoz_metrics.distributed_exp_hist ON CLUSTER {{.SIGNOZ_CLUSTER}} AS signoz_metrics.exp_hist ENGINE = Distributed("{{.SIGNOZ_CLUSTER}}", "signoz_metrics", exp_hist, cityHash64(metric_name, fingerprint));
