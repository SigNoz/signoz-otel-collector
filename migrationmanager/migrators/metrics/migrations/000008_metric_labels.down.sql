DROP TABLE IF EXISTS signoz_metrics.metric_labels ON CLUSTER {{.SIGNOZ_CLUSTER}};

DROP TABLE IF EXISTS signoz_metrics.distributed_metric_labels ON CLUSTER {{.SIGNOZ_CLUSTER}};