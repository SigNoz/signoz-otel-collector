ALTER TABLE signoz_metrics.time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS description;
ALTER TABLE signoz_metrics.distributed_time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS description;

ALTER TABLE signoz_metrics.time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS unit;
ALTER TABLE signoz_metrics.distributed_time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS unit;

ALTER TABLE signoz_metrics.time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS type;
ALTER TABLE signoz_metrics.distributed_time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS type;

ALTER TABLE signoz_metrics.time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS is_monotonic;
ALTER TABLE signoz_metrics.distributed_time_series_v3 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS is_monotonic;

ALTER TABLE signoz_metrics.time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS description;
ALTER TABLE signoz_metrics.distributed_time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS description;

ALTER TABLE signoz_metrics.time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS unit;
ALTER TABLE signoz_metrics.distributed_time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS unit;

ALTER TABLE signoz_metrics.time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS type;
ALTER TABLE signoz_metrics.distributed_time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS type;

ALTER TABLE signoz_metrics.time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS is_monotonic;
ALTER TABLE signoz_metrics.distributed_time_series_v2 ON CLUSTER {{.SIGNOZ_CLUSTER}}
    DROP COLUMN IF EXISTS is_monotonic;
