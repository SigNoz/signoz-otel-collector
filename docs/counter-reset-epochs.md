# Counter-reset epochs: write side

Status: implemented behind `enable_start_ts` (exporter) + schema migration 1012.
Read side: `docs/counter-reset-epochs.md` in the SigNoz repo (querier pipeline,
feature flag `use_counter_epochs`, verification harness).

## Problem

Cumulative monotonic counters reset when processes restart. The read path
detects resets by value drops between step buckets, which structurally cannot
be exact:

- a reset *inside* a bucket is hidden by `max(value)` (undercount, and the 5m /
  30m rollups bake the loss in permanently);
- a reset whose counter regrows past the previous value before the next export
  is invisible to value comparison at any granularity (silent undercount);
- results change with the step interval (the same query disagrees with itself
  between auto step and 1-day step);
- a series' first-ever point can't be charted at all.

OTLP already carries the missing information: `start_time_unix_nano` changes
exactly when the counter's accumulation restarted. The exporter used to drop
it.

## Design

**Epoch** = normalized start time (ms) of a cumulative monotonic stream,
written to the new `start_ts` column of `samples_v4`. `start_ts = 0` means
"epoch unknown" (old rows, gauges, delta, non-monotonic sums, sources without
a usable start time, no-recorded-value markers). Within one epoch the value
sequence is non-decreasing **by construction** — that is the invariant the
whole design rests on, and the normalizer's only job is to guarantee it.

Because per-(bucket, epoch) `min`/`max` of a monotone sequence are its first
and last observations, they are *lossless* for rate/increase — and they are
order-free, duplicate-proof, mergeable aggregate states. The rollup tables
carry them as map columns keyed by `start_ts`:

```
min_unix_milli_by_start_ts  SimpleAggregateFunction(minMap, Map(Int64, Int64))
min_value_by_start_ts       SimpleAggregateFunction(minMap, Map(Int64, Float64))
max_unix_milli_by_start_ts  SimpleAggregateFunction(maxMap, Map(Int64, Int64))
max_value_by_start_ts       SimpleAggregateFunction(maxMap, Map(Int64, Float64))
```

This is what makes reset-exact answers possible **from the rollups at any step
interval** — the piece that was previously believed impossible ("the table
type can only support functions that can be independently merged"; per-epoch
min/max maps are exactly such functions). The timestamp maps are not consumed
by the querier yet; they are there for epoch-overlap diagnostics and future
rate-denominator work, and cost a few bytes per bucket.

Key properties, all validated in the harness (SigNoz repo,
`tests/integration/testdata/counter_reset_epochs/`):

- merges are idempotent and commutative: late data, retries, duplicated insert
  batches, and part merges cannot corrupt the states;
- the 30m table chains off the 5m table with plain `minMap`/`maxMap` merges;
- maps deliberately include key 0, so pre-rollout rows and epoch rows coexist
  per bucket and the querier can seam them row-by-row (no rollout watermark).

## Normalizer (`epoch.go`)

The output is always **a validated wire value or 0 — never an invented
timestamp**. That single property is what makes the design safe under
load-balanced collector replicas with no series stickiness: a wire start is
data, so every replica that trusts it emits the same epoch; every distrust
decision emits 0. (A per-replica synthesized or pinned timestamp would split
one series into what the read path must treat as parallel writers, and
parallel writers are *summed* — ongoing overcounting. So synthesis is banned.)

Per-series state (keyed by fingerprint, 64 shards, 2h idle TTL, 10m sweep)
exists only to decide *when to distrust the wire*:

| situation | action |
|---|---|
| first sighting, valid wire start | adopt it (this is what makes a single-shot script's one point visible) |
| valid start, unchanged | keep epoch, count stability; a previously distrusted series re-earns the epoch once the start repeats |
| valid start changed + value dropped | genuine reset → adopt the new start |
| valid start changed + value grew + start was stable ≥ 2 points | restart that regrew past the previous value → adopt (the reset value-based detection can never see) |
| valid start changed + value grew + start not stable | churn guard → distrust to 0 (spec-violating SDKs advance start on every export; honoring that would shred a monotone series into per-point epochs) |
| value dropped without a start change | the start doesn't delimit monotone runs → distrust to 0 |
| start absent/invalid | 0 (no synthesis; resets in such series keep the legacy read-path semantics) |
| out-of-order sample | assign (wire start if valid, else current epoch) without touching state |
| NoRecordedValue marker | never reaches the normalizer (a fake 0 would read as a reset); written with `start_ts = 0`, and the read path filters flagged rows anyway |

Replica semantics, by source class:

- **Spec-compliant sources** (stable start, advanced on restart — OTel SDKs,
  prometheusreceiver via its metrics adjuster): epochs are pure functions of
  the wire data → **exact under any replica count and any load balancing**,
  including during restarts (all replicas adopt the same new wire value,
  skewed by at most their own next sample).
- **Churny sources**: at most the first 1–2 points per (replica, series,
  state-birth) carry a first-sight epoch before distrust kicks in — a bounded
  one-time transient — then 0/legacy until the start stabilizes.
- **Start-less sources**: always 0/legacy. Their resets are handled exactly as
  today; no improvement, no new risk.
- **State loss** (restart, eviction): re-adoption from the wire on first
  sighting — compliant sources converge instantly to the same value.

The asymmetry is deliberate: wrongly minting epochs overcounts (dangerous,
and under replicas *persistently*), wrongly withholding them reproduces
today's behavior (safe). What the normalizer cannot fix: sources with broken
start times AND reset-with-regrowth between two exports stay invisible — the
same blind spot Prometheus has without created timestamps. Never worse than
today.

Cost: ~64 bytes of state per active cumulative monotonic series per collector
(1M series ≈ 64 MB), one sharded-mutex map operation per cumulative sample —
noise next to the fingerprint hashing already done per sample.

Could it be dropped entirely (write the wire start verbatim)? Validation
aside, the churn guard is the difference between "a spec-violating SDK gets
legacy behavior" and "a spec-violating SDK gets per-point epochs and its rate
charts read ~sum-of-values". One bad source melting one tenant's dashboards is
not an acceptable failure mode, so: no.

## What gets an epoch

Cumulative + monotonic samples only: Sum data points, histogram `.count` /
`.sum` / `.bucket` series (each `le` series normalized independently, sharing
the datapoint's start time — so a mid-bucket restart keeps all bucket series
consistent and `histogramQuantile` over reset boundaries becomes exact), and
summary `.count` / `.sum`. Gauges, deltas, UpDownCounters, summary quantiles,
histogram `.min`/`.max` get 0. Cumulative exponential histograms are not
ingested at all today (delta-only), so sketches are out of scope until that
changes.

## Migration 1012

- `start_ts Int64 DEFAULT 0 CODEC(DoubleDelta, ZSTD(1))` on `samples_v4`,
  `distributed_samples_v4`, `samples_v4_buffer`, `distributed_samples_v4_buffer`
  (constant runs per series → compresses to almost nothing);
- the four map columns on `samples_v4_agg_{5m,30m}` (+ distributed);
- `samples_v4_agg_5m_mv` rebuilt with an inner subquery: the bucket alias and
  the raw sample timestamp are both needed, and aliasing
  `intDiv(...) AS unix_milli` in the same SELECT would shadow the source column
  inside the map expressions (the known MV alias landmine). Maps are built via
  `minMapIf/maxMapIf(..., temporality = 'Cumulative')` — gauges and delta rows
  don't pay for them;
- `samples_v4_agg_30m_mv` merges the 5m maps;
- `samples_v4_mv` (buffer → samples_v4, reduction deployments) forwards
  `start_ts`.

Down migration restores the previous MV queries and drops the columns.

## Rollout order

1. Run migration 1012 everywhere the exporter writes.
2. Set `enable_start_ts: true` on exporters (default false — the insert
   column list changes, so enabling before migrating would fail inserts).
3. Only then enable `use_counter_epochs` in SigNoz (per-org feature flag).

Mixed collector fleets: old collectors keep writing rows without `start_ts`
(→ 0 via column default), and the querier seams key-0 and epoch rows. In
steady state, replica count and load balancing are non-issues for
spec-compliant sources (epochs are wire values — see the normalizer section).
The one transition caveat: a series *alternating* between old and new
collector versions for a long time repeatedly crosses the key-0↔epoch seam
and can double-count there; prefer upgrading a series' path atomically
(per-agent / per-cluster) over percentage canaries, and keep the version-mix
window short.

## Cost

- `samples_v4`: +8 bytes/row before compression; delta-encoded constant runs
  in practice compress to well under 1 byte/row.
- rollups: map states with ~1 entry per bucket (`restarts within the bucket
  + 1`); worst case is bounded by samples/bucket for degenerate sources, and
  the churn guard prevents the degenerate case at the source.
- normalizer: ~64 bytes per active cumulative monotonic series per collector.

## Follow-ups

- The reduced-metrics 60s cumulative MV still uses per-point value-drop
  detection on the buffer; it could use `start_ts` (the buffer now carries it)
  to catch regrow-past-previous resets in the reduction path too.
- Exponential histogram sketches need their own per-epoch design whenever
  cumulative exp-hist ingestion lands.
