# Cascading Filter Processor

Supported pipeline types: traces

The Cascading Filter processor is a [fork of tailsamplingprocessor](../tailsamplingprocessor) which
allows for defining smart cascading filtering rules with preset limits.

## Processor configuration

The following configuration options should be configured as desired:
- `policies` (no default): Policies used to make a sampling decision
- `spans_per_second` (default = 1500): Maximum total number of emitted spans per second
- `probabilistic_filtering_ratio` (default = 0.2): Ratio of spans that are always probabilistically filtered 
(hence might be used for metrics calculation)

The following configuration options can also be modified:
- `decision_wait` (default = 30s): Wait time since the first span of a trace before making a filtering decision
- `num_traces` (default = 50000): Number of traces kept in memory
- `expected_new_traces_per_sec` (default = 0): Expected number of new traces (helps in allocating data structures)

## Policy configuration

Each defined policy is evaluated with order as specified in config. There are several properties:
- `name` (required): identifies the policy
- `spans_per_second` (default = 0): defines maximum number of spans per second that could be handled by this policy. When set to `-1`,
it selects the traces only if the global limit is not exceeded by other policies (however, without further limitations)

Additionally, each of the policy might have any of the following filtering criteria defined. They are evaluated for 
each of the trace spans. If at least one span matching all defined criteria is found, the trace is selected:
- `numeric_attribute: {key: <name>, min_value: <min_value>, max_value: <max_value>}`: selects span by matching numeric
attribute (either at resource of span level)
- `string_attribute: {key: <name>, values: [<value1>, <value2>]}`: selects span by matching string attribute that is one
of the provided values (either at resource of span level)
- `properties: { min_number_of_spans: <number>}`: selects the trace if it has at least provided number of spans
- `properties: { min_duration: <duration>}`: selects the span if the duration is greater or equal the given value 
(use `s` or `ms` as the suffix to indicate unit)
- `properties: { name_pattern: <regex>`}: selects the span if its operation name matches the provided regular expression

## Limiting the number of spans 

There are two `spans_per_second` settings. The global one and the policy-one.

While evaluating traces, the limit is evaluated first on the policy level and then on the global level. The sum
of all `spans_per_second` rates might be actually higher than the global limit, but the latter will never be
exceeded (so some of the traces will not be included).

For example, we have 3 policies: `A, B, C`. Each of them has limit of `300` spans per second and the global limit 
is `500` spans per second. Now, lets say, that there for each of the policies there were 5 distinct traces, each
having `100` spans and matching policy criteria (lets call them `A1, A2, ... B1, B2...` and so forth:

`Policy A`: `A1, A2, A3`
`Policy B`: `B1, B2, B3`
`Policy C`: `C1, C2, C3`

However, in total, this is `900` spans, which is more than the global limit of `500` spans/second. The processor
will take care of that and randomly select only the spans up to the global limit. So eventually, it might
for example send further only following traces: `A1, A2, B1, C2, C5` and filter out the others.

## Example

```yaml
processors:
  cascading_filter:
    decision_wait: 10s
    num_traces: 100
    expected_new_traces_per_sec: 10
    spans_per_second: 1000
    probabilistic_filtering_ratio: 0.1
    policies:
      [
          {
            name: test-policy-1,
          },
          {
            name: test-policy-2,
            numeric_attribute: {key: key1, min_value: 50, max_value: 100}
          },
          {
            name: test-policy-3,
            string_attribute: {key: key2, values: [value1, value2]}
          },
          {
            name: test-policy-4,
            spans_per_second: 35,
          },
          {
            name: test-policy-5,
            spans_per_second: 123,
            numeric_attribute: {key: key1, min_value: 50, max_value: 100}
          },
          {
            name: test-policy-6,
            spans_per_second: 50,
            properties: {min_duration: 9s }
          },
          {
            name: test-policy-7,
            properties: {
              name_pattern: "foo.*",
              min_number_of_spans: 10,
              min_duration: 9s
            }
         },
        {
          name: everything_else,
          spans_per_second: -1
        },
      ]
```

Refer to [cascading_filter_config.yaml](./testdata/cascading_filter_config.yaml) for detailed
examples on using the processor.
