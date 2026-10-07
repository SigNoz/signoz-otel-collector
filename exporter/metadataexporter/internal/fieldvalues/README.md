# Field values writer

The field values writer is part of the metadata exporter. It writes the field
values store, from which the query service reads plain values, related values,
and metric keys:

- `signoz_metadata.field_values_sets`: one row for each pair of each set (a
  set is the `field = value` pairs of a record, apart from its resource; for
  metrics, a set is one series), and one row for each resource field and
  value of each resource.
- `signoz_metadata.field_values_daily`: a view with one row per value per
  day, and the number of sets that hold the value.

Migration 1002 of the schema migrator creates both tables.

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
        pre_write_window: 1h    # 0 disables the spread of new windows and of the daily sample
      classification:
        refresh_interval: 15m
        lookback_days: 7
      always_include: []
```

## What the writer does

- **Sets.** Fields with few values make the hash of a set. A field with more
  than `max_record_field_values` values in a UTC day leaves the hash. Its
  values are still written, up to the limit plus one per collector per day
  (the sample).
- **Resources.** The resource fields with up to `max_resource_field_values`
  values per day make the identity of a resource. A resource field over the
  limit leaves the identity, but all its values are still written.
- **Limits per resource.** When a resource reaches `max_sets_per_resource` new
  sets in a step of a window, the field with the most values leaves its hash
  (a coarse set). The last step is one overflow set per resource. Pairs
  outside the hash are limited the same way.
- **Time.** Each row is written once per window per collector. A record time
  of 0, or more than 5 minutes in the future, becomes the time of the batch. A
  record time before the window becomes the start of the window, so a late
  record cannot stamp the window with the time of an earlier one.
- **Spread.** In the last `pre_write_window` of a window, a key seen again is
  also written for the next window, at its start. Each key is due from a time
  set by its hash, as in `pkg/timebucketedset`, so a new window does not start
  with a burst of all active sets. The daily sample of each high-cardinality
  field grows with the time of the day over the first `pre_write_window` of
  the UTC day.
- **Signals.**
  - Logs: attributes, severity, scope, and the paths of JSON bodies when the
    JSON config is enabled.
  - Traces: attributes, span fields, the fields that the traces exporter
    derives (`http_method`, `response_status_code`, and the rest), and event
    fields outside the hash.
  - Metrics: one set per series, with the id of `time_series_v4`. Histograms
    and summaries write only their `.count` series. Each metric label gets at
    least one row per day, so the keys of a metric are complete. Metrics have
    no value limits.
- **Classification.** Every `refresh_interval`, the writer reads
  `field_values_daily` for the fields over the limit today, and once per day
  for the closed days of the lookback and the fields of yesterday.

## Memory

Each writer (one signal of one exporter) has `max_bytes`, or its share of the
automatic size. Three quarters go to the window cache, a fixed table of 8-byte
keys of any size. A key of the window is never evicted: when a part is full,
new sets go to the overflow set (from the reserve), and then pairs are left
out until the next window. Keys written ahead give their slots to keys of the
window.

The last quarter is for the value tracker (8 to 16 bytes per value) and the
resource states. The values of a field are freed when the field passes its
limit and its sample is full. When this memory is full, new values are not
counted (the reads of `field_values_daily` and the coarse sets still bound the
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
| `signoz_metadata_exporter_field_values_keys_written_ahead` | keys written for the next window |
| `signoz_metadata_exporter_field_values_insert_errors` | failed inserts |
| `signoz_metadata_exporter_field_values_cache_collisions` | keys without a free slot in the local cache |
| `signoz_metadata_exporter_field_values_rows_skipped_shared` | rows that another collector already wrote in the window |
| `signoz_metadata_exporter_field_values_shared_cache_errors` | failed calls to the shared cache |
| `signoz_metadata_exporter_field_values_cache_keys`, `..._cache_capacity` | keys in the local cache, and its capacity, by `part`: `exact`, `reserve`, `ahead` |
| `signoz_metadata_exporter_field_values_tracker_bytes`, `..._tracker_capacity_bytes` | memory of the value tracker and the resource states, and its limit |

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
```
