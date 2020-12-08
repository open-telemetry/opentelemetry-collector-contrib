# Policy Sampling Processor

Supported pipeline types: traces

The policy sampling processor samples traces based on a set of defined policies. This processor makes the decision based on the received batch, making it ideal to work with the `groupbytrace` processor. To scale out this setup, use the `loadbalancer` exporter as a dedicated collector instance before the collector cluster containing the `groupbytrace` and this processor here:

```
 OpenTelemetry Collector #1              OpenTelemetry Collector #N
+--------------------------+           +----------------------------+
|                          |           |                            |
|  loadbalancer exporter   +---------->+   groupbytrace processor   |
|                          |           |              +             |
+--------------------------+           |              |             |
                                       |              v             |
                                       |                            |
                                       |  policy sampling processor |
                                       |              +             |
                                       |              |             |
                                       |              v             |
                                       |                            |
                                       |         exporters          |
                                       |                            |
                                       +----------------------------+
```

This processor is an alternative to the `tailsampling` processor when using the `groupbytrace` processor.

The following configuration options are required:
- `policies` (no default): Policies used to make a sampling decision
- `sample_undecided` (default: false): Whether to sample traces that weren't explicitly selected by any of the configured policies

Multiple policies exist today and it is straight forward to add more. These include:
- `always_sample`: Sample all traces
- `numeric_attribute`: Sample based on numberic attribute
- `string_attribute`: Sample based on string attribute
- `rate_limiting`: Sample based on rate

Examples:

```yaml
processors:
  policy_sampling:
    policies:
      [
          {
            name: test-policy-1,
            type: always_sample
          },
          {
            name: test-policy-2,
            type: numeric_attribute,
            numeric_attribute: {key: key1, min_value: 50, max_value: 100}
          },
          {
            name: test-policy-3,
            type: string_attribute,
            string_attribute: {key: key2, values: [value1, value2]}
          },
          {
            name: test-policy-4,
            type: rate_limiting,
            rate_limiting: {spans_per_second: 35}
         }
      ]
```

Spans will only be sent to the next consumer if they are explicitly part of a policy (`decision=sample`) and haven't been part of a policy's drop decision. Policies have a definitive decision, either positive (`sample`) or negative (`drop`), or an undecided outcome. Examples:
* the `always_sample` policy will always return a `decision=sample`
* the `numeric_attribute` will return a `decision=sample` if the attribute is found and its value is in the range, or `decision=undecided` otherwise
* the `string_attribute` will return a `decision=sample` if the attribute is found and its value is in the list, or `decision=undecided` otherwise
* the `rate_limiting` will return a `decision=undecided` if the number of spans per second is still within the limit, or `decision=drop` if the limit has been exceeded

This processor will evaluate all policies until one yields a `drop` decision. When no such decision is returned by any policies, the trace will be sampled if at least one policy returned a `sample` decision. When no `drop` nor `sample` decisions are made, the trace is seen as `undecided` and will be dropped.

## Metrics

The following metrics are exposed as part of this processor:

* `otelcol_policysampling_evaluation_latency_bucket`, with the `policy` and `success` (true|false) tags, indicating the latencies for evaluating a decision for the individual traces.
* `otelcol_policysampling_evaluation_outcome`, as a more convenient way to extract the `success` (true|false) of the individual policy's evaluation.
* `otelcol_policysampling_num_decisions`, aggegating the number of decisions per outcome (sampled|dropped|undecided).
