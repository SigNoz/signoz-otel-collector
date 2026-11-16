# SigNoz ClickHouse Audit Exporter

Writes SigNoz audit log records to the `signoz_audit` ClickHouse database.

## Configuration options

- `dsn` (default = `tcp://localhost:9000`): ClickHouse DSN. Driver options and ClickHouse settings go in its query
  string, see the [clickhouse-go DSN reference](https://pkg.go.dev/github.com/ClickHouse/clickhouse-go/v2#readme-dsn).
  When the DSN does not set them, the exporter adds:
  - `async_insert=true`
  - `compress=lz4`
  - `type_json_skip_duplicated_paths=1`, `input_format_try_infer_datetimes=0`, `input_format_try_infer_dates=0`

  Each push holds up to two connections, one per table. Size the pool for `sending_queue::num_consumers` with
  `max_open_conns` and `max_idle_conns`.
- `timeout`, `retry_on_failure`, `sending_queue`: see
  [exporterhelper](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md).

## Example config

```yaml
exporters:
  signozclickhouseaudit:
    dsn: tcp://default:password@127.0.0.1:9000?max_open_conns=25&max_idle_conns=21
    timeout: 10s
    sending_queue:
      num_consumers: 10
      queue_size: 1000
```

## Schema management

The exporter does not create tables. `signoz-otel-collector migrate` creates `signoz_audit.logs`,
`signoz_audit.logs_resource` and their `distributed_` tables; the exporter only inserts into the distributed ones.

## Internal telemetry

See [documentation.md](documentation.md).
