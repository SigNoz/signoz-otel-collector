# SigNoz DD Stats Processor

## Description

The SigNoz DD Stats Processor transforms Datadog APM stats payloads (sent as `dd.internal.stats.payload` metrics) into standard OpenTelemetry metrics format. This processor enables ingestion and processing of Datadog APM statistics while converting them to a format compatible with SigNoz's metrics pipeline.

## How It Works

The processor:

1. Scans incoming metrics for `dd.internal.stats.payload` metrics
2. Decodes the Datadog protobuf payload from the metric's attributes
3. Extracts APM statistics (hits, errors, duration) from the payload
4. Converts them into standard OTLP metrics with appropriate naming and attributes
5. Preserves original resource attributes and adds Datadog-specific metadata

## Converted Metrics

For each span operation in the Datadog stats payload, the processor generates the following metrics:

### Trace Metrics

- `trace.{name}.hits` - Total number of trace spans
- `trace.{name}.hits.by_http_status` - Hits grouped by HTTP status  - only when HTTP status code is present
- `trace.{name}.errors` - Total number of errors 
- `trace.{name}.errors.by_http_status` - Errors grouped by HTTP status  - only when errors > 0 and HTTP status code is present
- `trace.{name}` - Latency distribution (Histogram, Delta)

Where `{name}` is the operation name from `ClientGroupedStats.Name` (e.g., `order.create`, `db.query`, `cache.get`).

### Data Point Attributes

Each metric data point includes the following attributes (when available):

- `service` - Service name from the stats group
- `resource` - Resource name from the stats group
- `status_code` - HTTP status code (integer)
- `type` - Span type (e.g., web, db, cache, sql)
- `span.kind` - Span kind from the stats group

### HTTP Status Metrics Additional Attributes

Metrics with `.by_http_status` suffix include an additional attribute:

- `http.status_class` - HTTP status class (e.g., "2xx", "4xx", "5xx")

### Resource Attributes

The processor enriches resource attributes with Datadog-specific information:

- `host.name` - Hostname from the payload
- `deployment.environment` - Environment (e.g., prod, staging)
- `service.name` - Service name
- `service.version` - Service version
- `telemetry.sdk.language` - Language of the tracer
- `telemetry.sdk.version` - Tracer version
- `process.runtime.id` - Runtime ID
- `container.id` - Container ID

## Configuration

```yaml
processors:
  signozddstats:
    # Enable or disable the processor
    # Default: true
    enabled: true
```

### Configuration Options

- `enabled` (bool, default: `true`): Enable or disable the DD stats processing. When disabled, metrics pass through unchanged.

## Example Configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  signozddstats:
    enabled: true
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  signozclickhousemetrics:
    datasource: tcp://clickhouse:9000

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [signozddstats, batch]
      exporters: [signozclickhousemetrics]
```

## Use Cases

### Datadog Agent Integration

This processor is particularly useful when:

1. **Migrating from Datadog**: You want to continue using Datadog agents while transitioning to SigNoz

### Example Datadog Agent Configuration

Configure your Datadog agent to send APM stats to the OpenTelemetry collector:

```yaml
# datadog.yaml
apm_config:
  enabled: true
  apm_non_local_traffic: true
  
  # Send stats to OTLP endpoint
  otlp:
    enabled: true
    endpoint: http://otel-collector:4317
    
  # Enable stats aggregation
  compute_stats_by_span_kind: true
  peer_service_aggregation: true
```

## Performance Considerations

- The processor performs protobuf deserialization, which has minimal overhead
- Processing occurs only when `dd.internal.stats.payload` metrics are detected
- All other metrics are passed through (copied to output)
- The processor creates up to 5 OTLP metrics per stats group (hits, hits by status, errors, errors by status, duration histogram)
- Resource attributes from Datadog stats are added to each output metric's resource


### Debug Logging

Enable debug logging to troubleshoot issues:

```yaml
service:
  telemetry:
    logs:
      level: debug
```
