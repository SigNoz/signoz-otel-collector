# Clickhouse System Tables Receiver

Connects to ClickHouse to collect monitoring data from ClickHouse system tables.

The component can emit **both logs and metrics**:

- **Logs** - `system.query_log`, one log record per query (time-windowed cursor scrape).
- **Metrics** - `system.view_refreshes`, a periodic snapshot of refreshable
  materialized view health (staleness, last-refresh duration, exception, retry,
  progress) emitted as OTel gauges.

Each signal is independent: place the receiver in a logs pipeline, a metrics
pipeline, or both.

## Example Configuration

```yaml
receivers:
  clickhousesystemtablesreceiver:
    # required
    dsn: tcp://clickhouse:9000/

    # optional. Set to the cluster name when scraping a clustered ClickHouse
    # deployment; queries then target clusterAllReplicas(cluster_name, system.<table>)
    cluster_name: "cluster-name"

    # logs: scrape system.query_log
    query_log_scrape_config:
      scrape_interval_seconds: 20
      # required. min_scrape_delay_seconds must be larger than the
      # flush_interval_milliseconds setting for query_log so the rows are flushed
      # before they are scraped. See
      # https://clickhouse.com/docs/en/operations/server-configuration-parameters/settings#query-log
      min_scrape_delay_seconds: 8

    # metrics: snapshot system.view_refreshes (all refreshable MVs on the server)
    system_tables_metrics:
      collection_interval: 30s

service:
  pipelines:
    logs/clickhouse:
      receivers: [clickhousesystemtablesreceiver]
      exporters: [...]
    metrics/clickhouse:
      receivers: [clickhousesystemtablesreceiver]
      exporters: [...]
```

Individual metrics can be toggled under `system_tables_metrics.metrics.<metric>.enabled`
(standard mdatagen metric config). See `metadata.yaml` and `documentation.md`.

## Emitted metrics (`system.view_refreshes`)

All metrics carry the originating replica's hostname as the `clickhouse.hostname`
resource attribute, and `database` + `view` as datapoint attributes.

| Metric | Notes |
|---|---|
| `clickhouse.view_refresh.last_success_age` (s) | now() − last_success_time; primary staleness signal |
| `clickhouse.view_refresh.last_duration` (s) | duration of the last refresh; compare to cadence |
| `clickhouse.view_refresh.exception` (0/1) | 1 if the last refresh errored |
| `clickhouse.view_refresh.retry` | current retry count |
| `clickhouse.view_refresh.progress` (0..1) | progress of an in-flight refresh |
