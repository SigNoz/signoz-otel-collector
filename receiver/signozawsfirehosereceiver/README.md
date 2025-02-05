# AWS Kinesis Data Firehose Receiver

A fork of awsfirehosereceiver for powering AWS integration in SigNoz deployments. This fork will be upgraded independently from the general upstream updates of signoz-otel-collector.

awsfirehosereceiver is a receiver for ingesting AWS Kinesis Data Firehose delivery stream messages and parsing the records received based on the configured record type.

## Configuration

Example:

```yaml
receivers:
  signozawsfirehose:
    endpoint: 0.0.0.0:4433
    record_type: cwmetrics
    tls:
      cert_file: server.crt
      key_file: server.key
```
The configuration includes the Opentelemetry collector's server [confighttp](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#server-configuration),
which allows for a variety of settings. Only the most relevant ones will be discussed here, but all are available.
The AWS Kinesis Data Firehose Delivery Streams currently only support HTTPS endpoints using port 443. This can be potentially circumvented
using a Load Balancer.

### endpoint:
The address:port to bind the listener to.

default: `0.0.0.0:4433`

You can temporarily disable the `component.UseLocalHostAsDefaultHost` feature gate to change this to `0.0.0.0:4433`. This feature gate will be removed in a future release.

### tls:
See [documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#server-configuration) for more details.

A `cert_file` and `key_file` are required.

### record_type:
The type of record being received from the delivery stream. Each unmarshaler handles a specific type, so the field allows the receiver to use the correct one.

default: `cwmetrics`

See the [Record Types](#record-types) section for all available options.

## Record Types

### cwmetrics
The record type for the CloudWatch metric stream. Expects the format for the records to be JSON.
See [documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Metric-Streams.html) for details.

### cwlogs
The record type for the CloudWatch log stream. Expects the format for the records to be JSON.
For example:

```json
{
  "messageType": "DATA_MESSAGE",
  "owner": "111122223333",
  "logGroup": "my-log-group",
  "logStream": "my-log-stream",
  "subscriptionFilters": ["my-subscription-filter"],
  "logEvents": [
    {
      "id": "123",
      "timestamp": 1725544035523,
      "message": "My log message."
    }
  ]
}
```

### otlp_v1
The OTLP v1 format as produced by CloudWatch metric streams.
See [documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-opentelemetry-100.html) for details.
