# Tail Sampling Processor

Supported pipeline types: traces

The tail sampling processor samples traces based on a set of defined policies.
Today, this processor only works with a single instance of the collector.
Technically, trace ID aware load balancing could be used to support multiple
collector instances, but this configuration has not been tested. Please refer to
[config.go](./config.go) for the config spec.

The following configuration options are required:
- `policies` (no default): Policies used to make a sampling decision

Multiple policies exist today and it is straight forward to add more. These include:
- `always_sample`: Sample all traces
- `latency`: Sample based on the duration of the trace. The duration is determined by looking at the earliest start time and latest end time, without taking into consideration what happened in between.
- `numeric_attribute`: Sample based on number attributes
- `probabilistic`: Sample a percentage of traces. Read [a comparison with the Probabilistic Sampling Processor](#probabilistic-sampling-processor-compared-to-the-tail-sampling-processor-with-the-probabilistic-policy).
- `status_code`: Sample based upon the status code (`OK`, `ERROR` or `UNSET`)
- `string_attribute`: Sample based on string attributes value matches, both exact and regex value matches are supported
- `rate_limiting`: Sample based on rate
- `composite`: Sample based on a combination of above samplers, with ordering and rate allocation per sampler. Rate allocation allocates certain percentages of spans per policy order. 
  For example if we have set max_total_spans_per_second as 100 then we can set rate_allocation as follows
  1. test-composite-policy-1 = 50 % of max_total_spans_per_second = 50 spans_per_second
  2. test-composite-policy-2 = 25 % of max_total_spans_per_second = 25 spans_per_second
  3. To ensure remaining capacity is filled use always_sample as one of the policies

The following configuration options can also be modified:
- `decision_wait` (default = 30s): Wait time since the first span of a trace before making a sampling decision
- `num_traces` (default = 50000): Number of traces kept in memory
- `expected_new_traces_per_sec` (default = 0): Expected number of new traces (helps in allocating data structures)

Examples:

```yaml
processors:
  tail_sampling:
    decision_wait: 10s
    num_traces: 100
    expected_new_traces_per_sec: 10
    policies:
      [
          {
            name: test-policy-1,
            type: always_sample
          },
          {
            name: test-policy-2,
            type: latency,
            latency: {threshold_ms: 5000}
          },
          {
            name: test-policy-3,
            type: numeric_attribute,
            numeric_attribute: {key: key1, min_value: 50, max_value: 100}
          },
          {
            name: test-policy-4,
            type: probabilistic,
            probabilistic: {sampling_percentage: 10}
          },
          {
            name: test-policy-5,
            type: status_code,
            status_code: {status_codes: [ERROR, UNSET]}
          },
          {
            name: test-policy-6,
            type: string_attribute,
            string_attribute: {key: key2, values: [value1, value2]}
          },
          {
            name: test-policy-7,
            type: string_attribute,
            string_attribute: {key: key2, values: [value1, val*], enabled_regex_matching: true, cache_max_size: 10}
          },
          {
            name: test-policy-8,
            type: rate_limiting,
            rate_limiting: {spans_per_second: 35}
         },
         {
            name: test-policy-9,
            type: string_attribute,
            string_attribute: {key: http.url, values: [\/health, \/metrics], enabled_regex_matching: true, invert_match: true}
         },
         {
            name: composite-policy-1,
            type: composite,
            composite:
              {
                max_total_spans_per_second: 1000,
                policy_order: [test-composite-policy-1, test-composite-policy-2, test-composite-policy-3],
                composite_sub_policy:
                  [
                    {
                      name: test-composite-policy-1,
                      type: numeric_attribute,
                      numeric_attribute: {key: key1, min_value: 50, max_value: 100}
                    },
                    {
                      name: test-composite-policy-2,
                      type: string_attribute,
                      string_attribute: {key: key2, values: [value1, value2]}
                    },
                    {
                      name: test-composite-policy-3,
                      type: always_sample
                    }
                  ],
                rate_allocation:
                  [
                    {
                      policy: test-composite-policy-1,
                      percent: 50
                    },
                    {
                      policy: test-composite-policy-2,
                      percent: 25
                    }
                  ]
              }
          },
        ]
```

Refer to [tail_sampling_config.yaml](./testdata/tail_sampling_config.yaml) for detailed
examples on using the processor.

### Probabilistic Sampling Processor compared to the Tail Sampling Processor with the Probabilistic policy

The [probabilistic sampling processor][probabilistic_sampling_processor] and the probabilistic tail sampling processor policy work very similar:
based upon a configurable sampling percentage they will sample a fixed ratio of received traces.
But depending on the overall processing pipeline you should prefer using one over the other.

As a rule of thumb, if you want to add probabilistic sampling and...

...you are not using the tail sampling processor already: use the [probabilistic sampling processor][probabilistic_sampling_processor].
Running the probabilistic sampling processor is more efficient than the tail sampling processor.
The probabilistic sampling policy makes decision based upon the trace ID, so waiting until more spans have arrived will not influence its decision. 

...you are already using the tail sampling processor: add the probabilistic sampling policy.
You are already incurring the cost of running the tail sampling processor, adding the probabilistic policy will be negligible.
Additionally, using the policy within the tail sampling processor will ensure traces that are sampled by other policies will not be dropped.

[probabilistic_sampling_processor]: ../probabilisticsamplerprocessor
