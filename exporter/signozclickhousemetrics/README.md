# SigNoz ClickHouse Metrics Exporter

Exports OpenTelemetry metrics to the `signoz_metrics` database in ClickHouse.

The exporter writes a small set of landing tables; everything else in
`signoz_metrics` is derived from them by materialized views created by the
[schema migrator](../../cmd/signozschemamigrator). This README documents the
configuration and the full write-side data flow, so the table catalog can be
read as a map instead of a list.

## Configuration options

Required:

- `dsn` (default `tcp://localhost:9000`): ClickHouse DSN,
  e.g. `tcp://127.0.0.1:9000?username=user&password=pass`.

Optional:

- `database` (default `signoz_metrics`)
- `samples_table` (default `distributed_samples_v4`)
- `time_series_table` (default `distributed_time_series_v4`)
- `exp_hist_table` (default `distributed_exp_hist`)
- `metadata_table` (default `distributed_metadata`)
- `enable_exp_hist` (default `false`): write exponential histograms as DD
  sketches to the exp-hist table.
- `metadata_write_sample_ratio` (default `1.0`, range `(0, 1]`): fraction of
  attribute-metadata rows written per batch. Metadata rows are deduplicated
  within a batch; lowering the ratio additionally samples them, trading
  attribute-catalog completeness for fewer writes on extreme-ingest systems
  where the same attribute keys repeat across thousands of metrics.
- `timeout`, `retry_on_failure`, `sending_queue`: standard
  [exporterhelper](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper)
  settings.
- `reduction`: cardinality control (see below).
  - `enabled` (default `false`)
  - `poll_interval` (default `45s`, min `5s`): how often the rules table is
    re-read. Rules carry an `effective_from` set ahead by the writer, so poll
    cadence does not affect correctness within that margin.
  - `rules_table` (default `distributed_metric_reduction_rules`)
  - `buffer_samples_table` (default `distributed_samples_v4_buffer`)
  - `buffer_time_series_table` (default `distributed_time_series_v4_buffer`)

Always run a batch processor in front of this exporter.

## Example

```yaml
exporters:
  signozclickhousemetrics:
    dsn: tcp://clickhouse:9000
    timeout: 15s
    reduction:
      enabled: true
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [signozclickhousemetrics]
```

## Data flow

### Direct mode (`reduction.enabled: false`)

The exporter writes four table families directly; insert-time materialized
views derive the rest:

```
exporter --> samples_v4 --> samples_v4_agg_5m --> samples_v4_agg_30m      (pre-aggregates)
         --> time_series_v4 --> _6hrs --> _1day --> _1week                (series presence, hour to week epochs)
         --> exp_hist                                                     (if enable_exp_hist)
         --> metadata, usage                                              (attribute catalog, ingest accounting)
```

- `samples_v4`: one row per data point `(env, temporality, metric_name,
  fingerprint, unix_milli, value, flags)`. The fingerprint is a hash of the
  full label set.
- `time_series_v4`: one row per series per hour (`unix_milli` floored to the
  hour) carrying the labels; the `6hrs/1day/1week` tables re-bucket it to
  coarser epochs so long-range queries scan fewer rows per series. The query
  service picks a table by window size.
- Everything is written through `distributed_` tables sharded by
  `cityHash64(env, temporality, metric_name, fingerprint)`, so a series'
  samples and its time-series rows always land on the same shard.

### Cardinality control (`reduction.enabled: true`)

With reduction on, the buffer tables become the universal landing target and
the regular tables above are fed from them by materialized views. The direct
mode graph still exists downstream, unchanged:

```
exporter --> samples_v4_buffer ----> samples_v4 --> agg_5m --> agg_30m          (rows not under a rule)
         |                       |-> samples_v4_reduced_last_60s   --> _5m --> _30m  (gauges, non-monotonic cumulative)
         |                       |-> samples_v4_reduced_sum_60s    --> _5m --> _30m  (delta + cumulative counters, histograms)
         |
         --> time_series_v4_buffer ----> time_series_v4 --> _6hrs --> _1day --> _1week  (series not under a rule)
                                     |-> time_series_v4_reduced --> _1day                (reduced group catalog)
```

The exporter polls `rules_table` for reduction rules, keyed by the stored
(flattened) metric name i.e for histograms and summaries each derived series
(`.bucket`, `.sum`, `.count`, …) is reduced under its own name. A rule lists
label keys and a mode: drop the listed keys (keep the rest) or keep only the
listed keys (drop the rest). Protected labels `le`, `quantile`, `__name__`,
`__temporality__`, `deployment.environment` are never dropped. A rule's
`effective_from` is compared against each **datapoint's timestamp**, not the
wall clock, so all collector replicas start reducing at the same data-time
boundary regardless of when they poll.

For a datapoint the rule applies to, the exporter computes the **reduced
fingerprint** by re-running the fingerprint chain over the surviving labels:
reduce the resource attributes, seed the scope reduction with that hash, seed
the point reduction with the scope hash, then fold in the metric name. Any two
series identical in their kept labels at every level collapse to the same
reduced fingerprint and that shared identity is the reduced series ("group") the
raw series aggregates into. Three kinds of rows are written:

- **samples buffer**: every raw data point at full resolution, carrying its
  raw `fingerprint` plus the group's `reduced_fingerprint`;
- **ts buffer, raw series row** (`is_reduced = false`): the series' full label
  set, with `reduced_fingerprint` linking it to its group;
- **ts buffer, reduced catalog row** (`is_reduced = true`): the group's
  identity reduced fingerprint in both fingerprint columns, and the kept
  labels reduced per level (attrs, scope attrs, resource attrs). Emitted once
  per batch per group (deduplicated in-batch); like every ts row its
  `unix_milli` is floored to the hour on write.

Datapoints from *before* a rule's `effective_from` (and all unruled metrics)
get `reduced_fingerprint = 0`, and that marker is what the buffer-to-regular
MVs route on (`samples_v4_mv`: `WHERE reduced_fingerprint = 0`;
`time_series_v4_mv`: `WHERE is_reduced = false AND reduced_fingerprint = 0`).
So a ruled metric's history up to activation flows into the regular tables,
while its post-activation raw samples and full-label series rows exist only in
the buffers: they serve full-resolution reads for recent windows and expire
with the buffer TTL (~24h). Past that horizon only the reduced representation
remains per-group 60s aggregates and the kept-label catalog. That is the
feature working as intended: full fidelity is deliberately ephemeral for ruled
metrics, and queries older than the buffer window are answered from the
reduced tables.

The reduced tables are built from the buffer by **refreshable**
materialized views, not insert-time ones, because aggregating across series
needs a lookback window (cumulative to delta conversion needs each series'
previous sample; per-series `last` needs the whole minute):

| view | cadence | reads buffer window | emits whole 60s buckets in |
|---|---|---|---|
| `..._last_60s_mv` | 1m | `[now−11m, now−2m)` | `[now−10m, …)` |
| `..._sum_60s_delta_mv` | 1m | `[now−11m, now−2m)` | `[now−10m, …)` |
| `..._sum_60s_cumulative_mv` | 1m | `[now−11m, now−2m)` | `[now−10m, …)` (rewrites temporality to `Delta`) |
| `..._{last,sum}_5m_mv` | 5m | 60s tables `[now−40m, now−12m)` | `[now−35m, now−17m]` |
| `..._{last,sum}_30m_mv` | 30m | 5m tables `[now−180m, now−40m)` | `[now−150m, now−70m]` |

Three properties of this design that everything downstream relies on:

- **Versioned re-emission.** Each refresh rewrites every bucket still inside
  its window with a fresh `computed_at`; the tables are
  `ReplacingMergeTree(computed_at)` and readers take the newest version (the
  query service reads with `FINAL`). This makes refreshes at-least-once and
  self-healing: a missed refresh is covered by later ones while the bucket is
  in-window. Late data within the lookback is absorbed the same way.
- **Whole buckets only.** Every view guards both window edges so it never
  emits a bucket the scan only partially covers otherwise the newest version
  of a bucket could be a truncated one, served as truth.
- **Shard locality.** The reduced tables shard by `cityHash64(env,
  temporality, metric_name, reduced_fingerprint)`, matching the reduced ts
  catalog, so the views run shard-local and query-side joins never cross
  shards.

`time_series_v4_reduced` gets one row per reduced group per hour (insert-time
MV from the ts buffer, monotonic-cumulative rewritten to `Delta`);
`..._reduced_1day` re-buckets it to day epochs for long windows. Reduced
cardinality is bounded by construction, so it needs fewer epoch tiers than the
raw ladder.

### Freshness expectations

Derived tables lag by design: reduced 60s buckets finalize ~10m behind
real time, 5m buckets ~35m, 30m buckets ~2.5h. Recent windows are served from
the buffer, so this is invisible to queries but operators watching
`system.view_refreshes` or `max(unix_milli)` per table should expect these
lags. For repairing gaps after prolonged refresh outages, see the cardinality
control backfill runbook.
