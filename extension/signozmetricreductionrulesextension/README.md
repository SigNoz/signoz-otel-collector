# Metric reduction rules extension

Reads metric reduction rules from ClickHouse and serves them to other components as one shared snapshot.

The extension polls the rules table at `poll_interval` and keeps the latest row per metric. A failed poll keeps the last loaded rules. Start runs one poll before pipelines start and never fails on a poll error; with no rules, nothing is reduced.

Rows that are deleted, have an unknown `match_type`, drop no labels, or drop a protected label (`le`, `quantile`, `__name__`, `__temporality__`, `deployment.environment`) are skipped.

## Configuration

| Field | Default | Description |
| --- | --- | --- |
| `dsn` | `tcp://localhost:9000` | ClickHouse DSN. |
| `database` | `signoz_metrics` | Database of the rules table. |
| `rules_table` | `distributed_metric_reduction_rules` | Rules table. |
| `poll_interval` | `45s` | How often the rules table is read. Minimum `5s`. |

```yaml
extensions:
  signozmetricreductionrules:
    dsn: tcp://clickhouse:9000

service:
  extensions: [signozmetricreductionrules]
```

Internal metrics are listed in [documentation.md](documentation.md).
