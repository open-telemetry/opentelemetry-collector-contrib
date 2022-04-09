# Probabilistic Sampling Processor

Supported pipeline types: traces, logs

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

The probabilistic sampler supports sampling logs according to their trace ID, or by a specific log record attribute.

The probabilistic sampler optionally may use a `hash_seed` to compute the hash of a log record.
This sampler samples based on hash values determined by log records. In order for
log record hashing to work, all collectors for a given tier (e.g. behind the same load balancer)
must have the same `hash_seed`. It is also possible to leverage a different `hash_seed` at
different collector tiers to support additional sampling requirements. Please refer to
[config.go](./config.go) for the config spec.

The following configuration options can be modified:
- `hash_seed` (no default, optional): An integer used to compute the hash algorithm. Note that all collectors for a given tier (e.g. behind the same load balancer) should have the same hash_seed.
- `sampling_percentage` (default = 0, required): Percentage at which logs are sampled; >= 100 samples all logs
- `trace_id_sampling` (default = true, optional): Whether to use the log record trace ID to sample the log record.
- `sampling_source` (default = null, optional): The optional name of a log record attribute used for sampling purposes, such as a unique log record ID. The value of the attribute is only used if the trace ID is absent or if `trace_id_sampling` is set to `false`.
- `sampling_priority` (default = null, optional): The optional name of a log record attribute used to set a different sampling priority from the `sampling_percentage` setting. 0 means to never sample the log record, and >= 100 means to always sample the log record.

Examples:

Sample 15% of the logs:
```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 15
```

Sample logs according to their logID attribute:

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 15
    trace_id_sampling: false
    sampling_source: logID
```

Sample logs according to the attribute `priority`:

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 15
    sampling_priority: priority
```


Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.

