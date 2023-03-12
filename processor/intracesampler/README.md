# In-Trace Sampling Processor

| Status                   |                               |
| ------------------------ |-------------------------------|
| Stability                | [alpha]                       |
| Supported pipeline types | traces                        |
| Distributions            | [contrib]                     |

Supported pipeline types: traces

The in-trace sampler can sample (remove) individual spans from a complete trace when they are too costly / verbose / noisy.

To function properly, the sampler must receive "complete" traces from which it can derive contextual span properties used for sampling decisions, such as roots and leaves. It means that all spans for a given trace MUST be received by the same collector instance and that in-trace sampler processor MUST be placed in the trace pipeline after a processor that group spans by trace id (such as `tail_sampling` or `groupbytrace`)

Just like any other sampler, the in-trace sampler can be configured such that for each span it can either sample (keeps all spans in a trace and does not change it) or not-sample (remove some of the spans in the trace). The `sampling_percentage` config option is used to control how many traces are passed as is and how many are sampled, so the user can save examples of both.

The following configuration options can be modified:
- `scope_leaves` (default = empty list): list of scope names to remove if they are leaves in the trace tree.
- `hash_seed` (no default): An integer used to compute the hash algorithm. Use the same `hash_seed` between different processors to force consistent sampling behavior, or different values to make sampling decisions independent.
- `sampling_percentage` (default = 0): Percentage at which spans are sampled; >= 100 samples (do not modify) all traces. 0 means always applying sampling on relevant spans.

Examples:

```yaml
processors:
  in_trace_sampler:
    scope_leaves:
      - "@opentelemetry/instrumentation-net"
    sampling_percentage: 15.3
    hash_seed: 22
```

## Hashing

Sampling percentage is calculated based on trace id hash, and is compatible with `Probabilistic Sampling Processor` and other `In-Trace sampling processor`s in the same pipeline. Using the same `hash_seed` for different processors guarantee they are synced about sampling decision for the same trace - if the same `sampling_percentage` is used they will have the same decision and if one is higher than the other then it will be sampled if the other is sampled etc.
Use different `hash_seed` to get an independent sampling decision for different sampler processors.

## Unsampling

The in-trace sampler is a tail sampler - it operates on spans which were head-based recorded 
and aggregated in the OpenTelemetry collector. Thus, when receiveing a batch of spans that
represent a full trace, it can choose which spans to unsample (remove).

Deciding which spans to remove is via processor configuration. 
Currently, only a single option is available: removing spans based on their scope name
and only if they are leaves in the complete trace.

Since those spans are leaves, there is no fear or need to change the trace topology - 
each span still points to their original parent. If a span has a sampled descendant, 
it implies that the span might be interesting to trace this sampled operation, and thus it is kept.

If multiple `scope_leaves` are supplied, the sampler will remove entire leaf branches that 
contains spans from scope which should be unsampled.

## Config

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol
