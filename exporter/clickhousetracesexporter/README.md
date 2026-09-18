# ClickHouse Traces Exporter

This exporter writes OpenTelemetry traces to ClickHouse.

## Configuration options

The following settings are required:

- `datasource`: ClickHouse data source name, for example
  `tcp://127.0.0.1:9000/?database=signoz_traces`.

The following settings are optional:

- `timeout` (default `5s`): Timeout for each attempt to send data.
- `max_allowed_data_age_days` (default `15`): Drop spans older than the current
  time minus this many days. Increase this value deliberately when backfilling
  historical traces whose ClickHouse retention has already been extended.
- `sending_queue`: In-memory queue configuration.
- `retry_on_failure`: Retry and backoff configuration.

```yaml
exporters:
  clickhousetraces:
    datasource: tcp://127.0.0.1:9000/?database=signoz_traces
    max_allowed_data_age_days: 60
```

The age limit is an ingestion guardrail independent of ClickHouse retention.
Increasing it can create writes to older partitions and may increase ClickHouse
CPU, I/O, and merge activity during a historical backfill.
