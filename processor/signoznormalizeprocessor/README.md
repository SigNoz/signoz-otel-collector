# SigNoz Normalize Processor

| Status        |           |
| ------------- |-----------|
| Stability     | alpha: logs |

Normalizes log bodies into the JSON shape that the `body_v2` column of
`signoz_logs.logs_v2` expects, so the ClickHouse logs exporter can store every
body as a JSON object with a consistent `message` field.

It is the processor form of the `normalize` operator of the
`signozlogspipeline` processor. The operator runs inside a stanza pipeline,
which converts every record from pdata to stanza entries and back and emits
asynchronously. This processor applies the same rules directly on pdata, so it
costs one JSON parse per record instead of a full stanza round trip, keeps
`ConsumeLogs` synchronous, and can be enabled from a static collector config
without going through pipelines.

## What it does

For every log record whose body is not empty:

1. The body becomes a map.
   - A string body that parses as a JSON object (after unquoting a quoted
     string) becomes that object. Integers are kept as 64-bit integers.
   - Any other string becomes `{"message": <string>}`.
   - A map body is kept as is.
   - Any other body (number, bool, bytes, array) becomes `{"message": <body>}`.
2. The `message` field is normalized.
   - A `null` `message` is removed.
   - When `message` is missing, the first of `message_fields` (default `log`,
     `msg`) that is present is renamed to `message`.
   - When `message` is itself an object, its fields are lifted to the top level
     and the `message` key is removed. An inner `message` key survives the lift.
   - Strings that look like JSON but fail to parse, for example two concatenated
     objects, are kept as text under `message`.

Records with an empty body are left untouched.

## Dual ingestion

With `json_body_dual_ingestion: true` the processor stores the body it received,
before any of the changes above, in the record attribute
`__signoz_original_body__`:

- string bodies byte for byte, including the quotes of a quoted JSON string,
- map bodies serialized the same way the exporter serializes a map body for the
  legacy `body` column, so the restored column matches legacy ingestion exactly,
- other bodies as the original typed value.

The ClickHouse logs exporter with its own `json_body_dual_ingestion: true`
restores that attribute into the legacy `body` column, writes the normalized
body to `body_v2`, and strips the attribute before insert. Keep both flags in
sync. The attribute never reaches attribute columns or metadata tables.

## Configuration

```yaml
processors:
  signoznormalize:
    json_body_dual_ingestion: false
    message_fields: [log, msg]
```

| Field                      | Default | Description                                                                 |
| -------------------------- | ------- | --------------------------------------------------------------------------- |
| `json_body_dual_ingestion` | `false` | Stash the pre-normalization body in `__signoz_original_body__` for the exporter to restore into the legacy `body` column. |
| `message_fields`           | `[log, msg]` | Ordered fields; the first present one is renamed to `message` when `message` is missing. |

## Placement

Put it where the read path needs it:

- When queries run on `body_v2`, place it before the `signozlogspipeline`
  processors so user pipelines see the same body the explorer shows.
- When only dual ingestion is on and queries still read the legacy `body`,
  place it after the `signozlogspipeline` processors so user pipelines keep
  seeing the raw body.

```yaml
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [memory_limiter, signoznormalize, signozlogspipeline/user, batch]
      exporters: [clickhouselogsexporter]
```

## Telemetry

`signoz_normalize_processor_logs_processed` counts the records whose body was
normalized. Records with an empty body are not counted.
