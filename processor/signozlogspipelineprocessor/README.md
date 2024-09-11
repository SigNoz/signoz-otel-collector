# Signoz Logs Pipeline Processor

An otel processor for powering SigNoz logs pipelines.

The goal of this processor is to achieve a seamless logs pipelines experience for SigNoz users
while exposing the same interface as the logstransform processor in opentelemetry-collector-contrib

The implementation mostly borrows from, adapts and enhances the official logstransform processor (and stanza)
code to achieve goals of SigNoz pipelines

On success, query service should be able to add logs pipelines to otel collector config using
this processor without having to do much translation or augmentation of the user defined config

The following non-exhaustive list of capabilities/improvements on top of what logstransform provides are included
- sync ConsumeLogs i.e. ConsumeLogs should only return after the ConsumeLogs of the next processor in the pipeline has returned.
  This helps with reliability of logs processing through the collector.
  The otel-collector-contrib logstransform processor doesn't have a sync implementation for ConsumeLogs
- Enhanced addressing capabilities.
  - should be able to refer to top level otlp log fields (like severity) in operator conditions
  - should be able to use body.field in operator config if body is JSON without having to first parse body into an temporary attribute