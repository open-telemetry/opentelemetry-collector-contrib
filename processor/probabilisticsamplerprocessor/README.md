# Probabilistic Sampling Processor

Supported pipeline types: traces, logs

The probabilistic sampler supports sampling logs by associating a sampling rate to log severity.

A default sampling rate is mandatory. Additionally, additional sampling rates associated with a log severity can be added.
Any message with a log severity equal or higher to the log severity will adopt the new sampling.

The probabilistic sampler optionally may use a `hash_seed` to compute the hash of a log record.
This sampler samples based on hash values determined by log records. In order for
log record hashing to work, all collectors for a given tier (e.g. behind the same load balancer)
must have the same `hash_seed`. It is also possible to leverage a different `hash_seed` at
different collector tiers to support additional sampling requirements. Please refer to
[config.go](./config.go) for the config spec.

The following configuration options can be modified:
- `hash_seed` (no default): An integer used to compute the hash algorithm. Note that all collectors for a given tier (e.g. behind the same load balancer) should have the same hash_seed.
- `sampling_percentage` (default = 0): Percentage at which logs are sampled; >= 100 samples all logs
- `severity/severity_level`: `SeverityText` associated with a [severity level](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#displaying-severity)
- `severity/sampling_percentage` (default = 0): Percentage at which logs are sampled when the severity is equal or higher to the severity text; >= 100 samples all logs
Examples:

```yaml
processors:
  probabilistic_sampler:
    hash_seed: 22
    sampling_percentage: 15
    severity:
      - severity_level: error
        sampling_percentage: 100
      - severity_level: warn
        sampling_percentage: 75
```

The probabilistic sampler supports two types of sampling for traces:

1. `sampling.priority` [semantic
convention](https://github.com/opentracing/specification/blob/master/semantic_conventions.md#span-tags-table)
as defined by OpenTracing
2. Trace ID hashing

The `sampling.priority` semantic convention takes priority over trace ID hashing. As the name
implies, trace ID hashing samples based on hash values determined by trace IDs. In order for
trace ID hashing to work, all collectors for a given tier (e.g. behind the same load balancer)
must have the same `hash_seed`. It is also possible to leverage a different `hash_seed` at
different collector tiers to support additional sampling requirements. Please refer to
[config.go](./config.go) for the config spec.

The following configuration options can be modified:
- `hash_seed` (no default): An integer used to compute the hash algorithm. Note that all collectors for a given tier (e.g. behind the same load balancer) should have the same hash_seed.
- `sampling_percentage` (default = 0): Percentage at which traces are sampled; >= 100 samples all traces

Examples:

```yaml
processors:
  probabilistic_sampler:
    hash_seed: 22
    sampling_percentage: 15.3
```

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.

