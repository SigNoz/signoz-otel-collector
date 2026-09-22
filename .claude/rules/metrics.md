---
paths:
  - "**/metadata.yaml"
  - "**/*telemetry*.go"
  - "**/*metrics*.go"
---

# Internal metrics

Metrics a component or shared package emits about itself. Rules distilled from OTel semconv [naming](https://opentelemetry.io/docs/specs/semconv/general/naming/) and [metrics](https://opentelemetry.io/docs/specs/semconv/general/metrics/), the collector [internal telemetry](https://opentelemetry.io/docs/collector/internal-telemetry/) docs and contrib practice. Worked example: `pkg/timebucketedset/metadata.yaml`.

## Define

- Declare every metric in `metadata.yaml` under `telemetry.metrics`; use the generated `internal/metadata.TelemetryBuilder`. Never hand-roll `meter.Int64Counter(...)`.
- Generate with the collector version pinned in `go.mod`: build `cmd/mdatagen` from a checkout of that tag; `go run ...@version` fails on its replace directives.
- Per metric: `enabled: true`, `stability: {level: alpha}`, `prefix: otelcol.`, `description`, `unit`, one of `sum` / `gauge` / `histogram`.
- Keys under `attributes` and `telemetry.metrics` sorted alphabetically; mdatagen rejects otherwise.
- Generated files stay as emitted.

## Name

- `otelcol.<type>.<namespace>.<leaf>`: dotted, lowercase, `_` inside a segment. `<type>` is the `type:` key. Namespaces singular: `bucket.count`, not `buckets.count`.
- Words come from the code's own API (`plan.ids`, `apply.ids`, `bucket.evictions`), never new vocabulary. If a word appears only once in the package, it is not vocabulary.
- Counter of discrete things: plural leaf (`bucket.evictions`, `plan.ids`). UpDownCounter or gauge of current state: `.count` (`bucket.count`). Amount used of a known total: `.usage`; the total: `.limit`. Elapsed: `.duration`.
- No `_total`, no unit in the name. The collector's Prometheus exporter keeps the dotted name; text exposition escapes dots to `_` unless the scraper negotiates UTF-8 names, so `otelcol.x.plan.ids` usually scrapes as `otelcol_x_plan_ids`.
- Units are UCUM: singular annotations `{id}`, `{bucket}`; `By`; `s`; utilization `1`.

## Attribute or separate metric

- One metric with an attribute when the values partition the same measurement and the sum over them is meaningful: `plan.ids{result=miss|hit|pre_write|no_bucket}`.
- Separate metrics for different concepts: state vs events (`bucket.count` vs `bucket.evictions`), `usage` vs `limit`, different instrument types.
- Attribute values are bounded enums or the caller's identity (`exporter` / `processor` / `receiver` = `component.ID.String()`). Never a rotating value: timestamp, bucket start, id.
- Same attribute key with different enums on two metrics: declare one yaml attribute per metric (`plan_result`, `apply_result`) with `name_override: result`.

## Record

- Build `attribute.Set` / `metric.WithAttributeSet` once in the constructor. The hot path allocates nothing.
- Per-row hot paths: `atomic.Int64` plus `async: true` instruments observed from `RegisterXxxCallback`. A sync `Add` locks the SDK aggregator on every call.
- Shared packages take `component.TelemetrySettings` and the caller's identifier `attribute.KeyValue`s, and expose `Shutdown()` to unregister callbacks.
- Test with `componenttest.NewTelemetry()` and the generated `metadatatest.AssertEqual*` helpers, `metricdatatest.IgnoreTimestamp()`.
