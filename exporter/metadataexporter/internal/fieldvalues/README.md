# Field values writer

The field values writer is part of the metadata exporter. It writes the field
keys and the field values store, from which the query service reads the keys
of a signal, plain values and related values:

- `signoz_metadata.field_keys_v2`: one row for each field key (signal,
  source, metric name, field context, field name, field data type). The key
  list must be complete, so the keys do not come from the rows of the values:
  see "Keys" below. The table has no partition: a key is written again each
  day, and ReplacingMergeTree makes one row for each key only inside a
  partition. A key is deleted 90 days after its last record.

- `signoz_metadata.field_values_sets`: one row for each pair of each set (a
  set is the `field = value` pairs of a record, apart from its resource; for
  metrics, a set is one series), and one row for each resource field and
  value of each resource. The table is partitioned by the week of each row,
  so reads of a short window skip older weeks and TTL drops whole weeks.
  Its projection `p_field_values` has one row per field, metric, value and
  UTC day in each part, for the reads that need one row per value: the values
  of a field with no filter, the values of a label for all metrics, and the
  classifier. A projection is stored in the part of its table, so it adds no
  part. An AggregatingMergeTree table needs
  `deduplicate_merge_projection_mode = 'rebuild'` for it, and each merge then
  computes the projection again.
- `signoz_metadata.field_days_over_limit`: one row per field and UTC day on
  which the field had more values than its limit.

Migration 1003 of the schema migrator creates the tables.

## Configuration

The writer is off by default. With `field_values.enabled: false`, the metadata
exporter does exactly what it did before. The writer runs only when the
exporter itself is enabled.

```yaml
exporters:
  metadataexporter:
    enabled: true
    tenant_id: <tenant>
    cache:
      redis:                    # used only when field_values.cache.provider is redis
        addr: redis:6379
    field_values:
      enabled: true
      source: ""                # the space inside the signal, such as "meter"
      limits:
        max_record_field_values: 5000
        max_resource_field_values: 100000
        max_value_bytes: 256
        max_fields_per_signal: 4096
        max_sets_per_resource: 16384
        max_outside_pairs_per_resource: 16384
      cache:
        provider: in_memory     # or redis
        max_bytes: 0            # 0: the writers of the process share 10% of the Go memory limit
        reserve_share: 0.1
        window: 24h             # each set, pair and resource is written once per window; divides 24h
        pre_write_window: 1h    # the start of a window over which its writes are spread; 0 disables it and the spread of the daily sample
      classification:
        refresh_interval: 15m
        lookback_days: 7
      always_include: []
      async_insert_busy_timeout: 1s   # 0 sends sync inserts
```

## What the writer does

- **Sets.** Fields with few values make the hash of a set. A field with more
  than `max_record_field_values` values in a UTC day leaves the hash. Its
  values are still written, up to the limit plus one per collector per day
  (the sample).
- **Types.** Resource and scope attributes are written as strings, whatever
  their type in the data, because the logs and traces exporters store them
  as strings. Record attributes keep their type.
- **Resources.** The resource fields with up to `max_resource_field_values`
  values per day make the identity of a resource. A resource field over the
  limit leaves the identity, but all its values are still written.
  `service.name` has no limit: it always stays in the identity, because most
  reads filter on it.
- **Limits per resource.** When a resource reaches `max_sets_per_resource` new
  sets in a step of a window, the field with the most values leaves its hash
  (a coarse set). The last step is one overflow set per resource. A field of
  `always_include` is never the field that leaves. When the pairs outside the
  hash reach `max_outside_pairs_per_resource` and no field can leave, the
  sets stay exact and only the new pairs outside the hash go to the overflow
  set. A resource counts its sets and pairs when their insert succeeds, so a
  failed insert does not use up its limits.
- **Time.** Each row is written once per window per collector. A record time
  of 0, or more than 5 minutes in the future, becomes the time of the batch. A
  record time before the window becomes the start of the window, so a late
  record cannot stamp the window with the time of an earlier one.
- **Spread.** The window of a key starts later than the window of the clock,
  by a time under `pre_write_window` that the hash of the key sets. With the
  defaults, a set whose time is 00:40 is written once between 00:40 UTC and
  00:40 UTC of the next day. A new day then does not start with a burst of
  all active sets, and a set seen at the end of a day gets no row of the next
  day. The costs:
  - A set that is new before its time is written at once, and one more time
    after its time. A collector that starts in the first `pre_write_window`
    of a day writes most of its sets two times.
  - A set of yesterday whose only records of today come before its time has
    no row of today. A read that starts today does not find it; a read that
    starts yesterday does.
  - The sets of yesterday count again for the limits of their resource, in
    the order of their times. A set that is new before its time counts two
    times.

  The daily sample of each high-cardinality field grows with the time of the
  day over the first `pre_write_window` of the UTC day.
- **Signals.**
  - Logs: attributes, severity, scope, and the paths of JSON bodies when the
    JSON config is enabled.
  - Traces: attributes, span fields, the fields that the traces exporter
    derives (`http_method`, `response_status_code`, and the rest), and event
    fields outside the hash.
  - Metrics: one set per series of a resource identity. A resource field over
    the limit leaves the identity as for logs and traces, so the series of the
    resources that differ only in that field (one resource per process, for
    example) are one series. Histograms and summaries write only their
    `.count` series. The labels of a new series follow the value rules of logs
    and traces, for each metric: a label with more than
    `max_record_field_values` values on a metric leaves the sets of that
    metric, which are then the labels that stay, and keeps a daily sample.
    The same label of another metric is not changed. The states of these
    counts have no limit of places. They get half of the tracker memory at
    most, at about 160 bytes for each metric and label: 6,075 metrics with 33
    labels each need 32 MB. A label with no state is not counted and stays in
    the set, unless the classifier has it over the limit.
    Such series are not cached, so a label with a value per point cannot fill
    the cache. The rows of a resource are written once, with no metric name. A
    link row (no metric name, the field `__name__`, the metric name as value)
    ties each metric to each of its resources. A label that leaves the sets
    with no sample has no row here; it is a key of its metric in
    `field_keys_v2`.
- **Keys.** The key of a pair is recorded before any rule above, so an empty
  value, a long value, a field with no place, a full sample, a full tracker
  and a full key cache cannot leave a key out. NaN and the infinities have a
  key and no value.
  - A key has the name under which the exporter of the signal stores the
    field. The logs exporter stores a map attribute as one JSON string, so
    its key is the name of the attribute. The traces exporter stores a map
    attribute of a span as one attribute for each leaf, so the keys have
    dotted names.
  - Data types: `string`, `bool`, `int64` or `float64` for the attributes of
    a log record or a span, and `string` for resource, scope and event
    fields, which the exporters store as strings. The exporters store `int64`
    and `float64` in one number column, and their own key tables have
    `float64` for both.
  - The fields that the writer makes from columns (`severity_text`,
    `kind_string`, `scope.name` and so on) are keys too, with the context
    `log`, `span` or `scope`.
  - The labels and scope fields of a metric are keys with the metric name.
    They are recorded when a series is written, which is once a day. The
    resource fields of a metric are recorded when the link of the metric to a
    resource is written. The resource fields are also keys with no metric
    name.
  - `source` separates the spaces of a signal: `audit` for audit logs and
    `meter` for meter metrics.
  - Keys that have no row of today wait in memory. A loop inserts them each
    15 seconds, async and **with** wait, and for the shards in the same call.
    A key counts as written only when that insert passes. After a failed
    insert, the keys wait for the next flush. The loop is not on the path of a
    push.
  - Each key is written again once per UTC day, which moves its `last_seen`.
  - The writer remembers about one million keys with a row of today. Past
    that, a key is written again in each flush in which it is seen, and no
    key is left out.
  - About four million keys can wait for an insert. A flush starts early when
    many keys wait, so only inserts that fail for a long time fill the list.
    A key that finds the list full is counted
    (`signoz_metadata_exporter_field_keys_not_queued`) and waits for its next
    record. For a label of a metric, that is the next day.
  - The last flush at the stop of the collector has 5 seconds.
  - Not the same as the key tables of the exporters: the logs exporter leaves
    out keys that look random (`keycheck.IsRandomKey`), and the traces
    exporter leaves out a span attribute that is NaN. Both are keys here.
  - The keys of JSON bodies are not written here. They stay in
    `signoz_metadata.field_keys`, which has their array types.
- **Inserts.** One insert per push, async by default: the server joins the
  inserts of `async_insert_busy_timeout` into one part per table and shard,
  and the shards join the inserts that the distributed table sends them. The
  writer does not wait for the flush, so the export does not wait either; a
  failed flush is not seen, and its keys stay cached with no rows until the
  window ends. Clusters with more shards need a longer timeout.
- **Classification.** A field is over its limit when
  `field_days_over_limit` has a row for it on one of the `lookback_days`,
  today included. Without the lookback, a field with a value per record
  starts each day inside the set identity. Every `refresh_interval`, the
  writer reads those rows. It then reads from the projection the other fields
  with more values than their limit today, and writes a row for each. The
  fields that have a row are left out of that read, as they hold most of the
  values. A writer whose own count of a field passes the limit also writes
  the row of the day, so a field that stays over its limit has a row for each
  day. In the first `refresh_interval` of a UTC day, the writer does the read
  one more time for the day that ended. The first refresh interval and the
  refresh after a day change start after a random time, so the collectors of
  a tenant do not read together. The start of the writer waits up to 5 s for
  the first read.

## Reads

- Values of a resource field for a metric: the resource rows (no metric
  name) whose `resource_hash` is in the link rows of the metric.
- Metrics of a resource: the values of the link rows whose `resource_hash` is
  in the resource rows that match.
- Values of a field with no filter: `GROUP BY` the value columns with the
  condition `toDate(last_seen, 'UTC') >= day`, which is the shape of the
  projection. ClickHouse reads the rows of the sets, with no notice, for a
  statement with a condition on `last_seen` itself or on a column that the
  projection does not have. With a metric name, ClickHouse can read the rows
  of the metric in place of the projection when they are fewer, so such a
  read must not set `force_optimize_projection`.
- The rows of the field `__name__` are links, not values: a read of values
  leaves them out.
- Keys: `GROUP BY` the key columns of `field_keys_v2`. A read that checks if
  a key is there must not filter on `last_seen`. The keys of one metric are
  the rows with its metric name. The values table has `number` where a key
  has `int64` or `float64`.
- A read must start at least one day before the time it asks for. In the
  first `pre_write_window` of a UTC day, most sets of the day before have no
  row of the new day yet, and a set that stops in that time gets none.
- Reads of distributed tables with `IN` subqueries need
  `distributed_product_mode = 'local'`, and an alias on each distributed table
  with the old analyzer.

## Memory

Each writer (one signal of one exporter) has `max_bytes`, or an equal share of
the automatic size. Three quarters go to the key cache: `pkg/timebucketedset`
with two buckets, one per window. Each bucket has at least 32 MiB. The writer
counts the keys of each window and stops at 80% of a bucket, before the
bucket drops its oldest keys: new sets then go to the overflow set (from the
reserve), and then pairs are left out until the next window.

The writers without `max_bytes` share their keys. Each one keeps half of its
key cache as its own floor; the other halves make one pool, which the writers
draw on as they need keys in the window. The signals of a tenant need very
different numbers of keys, so no fixed split fits all tenants. With 1 GiB of
automatic memory and three writers, each writer has a floor of 4.5M keys, and
any one of them can reach 17.9M.

The key cache uses more memory than its share. A bucket keeps its keys outside
the Go heap (12 bytes per key), and an index of 25 to 40 bytes per key on the
Go heap. Both buckets stay full all day: the bucket of the last window lives
until the bucket of the next window starts. Measured, a key cache share of
64 MiB uses about 220 MiB, and 192 MiB about 510 MiB. Plan about 3 times the
key cache share.

The last quarter is for the value tracker (8 to 16 bytes per value) and the
resource states. The values of a field are freed when the field passes its
limit and its sample is full. When this memory is full, new values are not
counted (the reads of the classifier and the coarse sets still bound the
field), and records of new resources go into their overflow set.

The work on the pdata (pairs, hashes, metric fingerprints) runs before the
writer lock. Under the lock, only the value rules and the cache are left.

With `provider: redis`, a shared cache is added. Before each insert, the
writer drops the rows whose keys another collector already wrote in the
window. The memory and the limits stay per collector. If Redis fails, the rows
are written anyway.

## Telemetry

| Metric | Meaning |
|---|---|
| `signoz_metadata_exporter_field_values_rows_written` | rows inserted |
| `signoz_metadata_exporter_field_values_left_out` | pairs not written, by `reason`: `value_length`, `field_places`, `sample_budget` (every pair of a field after its sample is full), `cache_full`, `tracker_full` |
| `signoz_metadata_exporter_field_values_resources_overflowed` | resources that started to use their overflow set in the window |
| `signoz_metadata_exporter_field_values_resources_untracked` | resources of a batch without a state, because the tracker memory is full |
| `signoz_metadata_exporter_field_keys_written` | rows written to `field_keys_v2` |
| `signoz_metadata_exporter_field_keys_insert_errors` | failed inserts of keys; the keys wait and are inserted again |
| `signoz_metadata_exporter_field_keys_not_queued` | keys seen while the list of waiting keys was full |
| `signoz_metadata_exporter_field_keys` | keys in memory, by `state`: `written` (a row of today), `waiting` |
| `signoz_metadata_exporter_field_values_insert_errors` | failed inserts. A failed flush of an async insert is not counted: it shows only as `FlushError` in `system.asynchronous_insert_log` |
| `signoz_metadata_exporter_field_values_classifier_errors` | failed reads or writes of the fields over the limit |
| `signoz_metadata_exporter_field_values_classification_time` | Unix time of the last read of the fields over the limit that passed, or 0 |
| `signoz_metadata_exporter_field_values_rows_skipped_shared` | rows that another collector already wrote in the window |
| `signoz_metadata_exporter_field_values_shared_cache_errors` | failed calls to the shared cache |
| `signoz_metadata_exporter_field_values_cache_keys`, `..._cache_capacity` | keys in the local cache, and its capacity, by `part`: `exact`, `reserve` |
| `signoz_metadata_exporter_field_values_tracker_bytes`, `..._tracker_capacity_bytes` | memory of the value tracker and the resource states, and its limit |
| `otelcol.timebucketedset.*` | the key cache: plans by result, applied keys, buckets, evictions (see `pkg/timebucketedset`) |

## Tests

```sh
# unit tests and benchmarks
go test ./exporter/metadataexporter/...
go test ./exporter/metadataexporter/internal/fieldvalues/ -run '^$' -bench .

# integration tests on a ClickHouse server with a cluster; -p 1 because the
# tests of each package recreate the signoz_metadata database
FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:9000 FIELDVALUES_CLICKHOUSE_CLUSTER=<cluster> \
  go test -p 1 -run Integration ./exporter/metadataexporter/...

# load tests: the cost with field_values off and on, and the reads
FIELDVALUES_PERF=1 FIELDVALUES_PERF_BATCHES=150 FIELDVALUES_PERF_REPEATS=3 \
FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:9000 FIELDVALUES_CLICKHOUSE_CLUSTER=<cluster> \
  go test -p 1 -run 'TestPerf' -v -timeout 90m ./exporter/metadataexporter/...

# churn test: days of Kubernetes-shaped logs and metrics where deployments roll
# each day; rows read for short windows, TTL, and the layouts of metric
# resource rows
FIELDVALUES_PERF=1 FIELDVALUES_CHURN_DAYS=35 FIELDVALUES_CHURN_PODS=300 FIELDVALUES_CHURN_PERCENT=20 \
FIELDVALUES_CLICKHOUSE_DSN=tcp://localhost:9000 FIELDVALUES_CLICKHOUSE_CLUSTER=<cluster> \
  go test -run TestPerfChurn -v -timeout 60m ./exporter/metadataexporter/internal/fieldvalues/
```
