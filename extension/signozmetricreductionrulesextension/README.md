# Metric reduction rules extension

Reads metric reduction rules from ClickHouse and serves them to other components as one shared snapshot.

The extension polls the rules table at `poll_interval` and keeps the latest row per metric. A failed poll keeps the last loaded rules. Start runs one poll before pipelines start and never fails on a poll error; with no rules, nothing is reduced. With `enabled: false`, the extension never polls and serves no rules.

The `signozclickhousemetrics` exporter and the `signozmeter` connector read `enabled` and the rules from the extension named in their `extension` field. Name it in both, so the meter subtracts what the exporter condenses.

Rows that are deleted, have an unknown `match_type`, drop no labels, or drop a protected label (`le`, `quantile`, `__name__`, `__temporality__`, `deployment.environment`) are skipped.

## Configuration

| Field | Default | Description |
| --- | --- | --- |
| `enabled` | `false` | The only reduction switch. When false, nothing is reduced or subtracted from the meter. |
| `dsn` | `tcp://localhost:9000` | ClickHouse DSN. |
| `poll_interval` | `45s` | How often the rules table is read. Minimum `5s`. |

```yaml
extensions:
  signozmetricreductionrules:
    enabled: true
    dsn: tcp://clickhouse:9000

exporters:
  signozclickhousemetrics:
    extension: signozmetricreductionrules

connectors:
  signozmeter:
    extension: signozmetricreductionrules

service:
  extensions: [signozmetricreductionrules]
```

Internal metrics are listed in [documentation.md](documentation.md).
