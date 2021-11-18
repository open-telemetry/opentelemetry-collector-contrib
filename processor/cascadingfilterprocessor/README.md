# Cascading Filter Processor

Supported pipeline types: traces

The Cascading Filter processor is a fork of
[tailsamplingprocessor][tailsamplingprocessor] which allows for defining smart
cascading filtering rules with preset limits.

[tailsamplingprocessor]:https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor

## Processor configuration

The following configuration options should be configured as desired:
- `trace_reject_rules` (no default): policies used to explicitly drop matching traces
- `trace_accept_rules` (no default): policies used to pass matching traces, within a specified limit
- `spans_per_second` (no default): maximum total number of emitted spans per second. 
When set, the total number of spans each second is never exceeded. This value can be also calculated
automatically when `probabilistic_filtering_rate` and/or `trace_accept_rules` are set
- `probabilistic_filtering_rate` (no default): number of spans that are always probabilistically filtered 
(hence might be used for metrics calculation). 
- `probabilistic_filtering_ratio` (no default): alternative way to specify the ratio of spans which  
are always probabilistically filtered (hence might be used for metrics calculation). The ratio is
specified as portion of output spans (defined by `spans_per_second`) rather than input spans. 
So filtering rate of `0.2` and max span rate of `1500` produces at most `300` probabilistically sampled spans per second.

The following configuration options can also be modified:
- `decision_wait` (default = 30s): Wait time since the first span of a trace before making a filtering decision
- `num_traces` (default = 100000): Max number of traces for which decisions are kept in memory
- `expected_new_traces_per_sec` (default = 0): Expected number of new traces (helps in allocating data structures)

Whenever rate limiting is applied, only full traces are accepted (if trace won't fit within the limit, 
it will never be filtered). For spans that are arriving late, previous decision are kept for some time.

## Updated span attributes

The processor modifies each span attributes, by setting following two attributes:
- `sampling.rule`: describing if `probabilistic` or `filtered` policy was applied
- `sampling.probability`: describing the effective sampling rate in case of `probabilistic` rule. E.g. if there were `5000`
spans evaluated in a given second, with `1500` max total spans per second and `0.2` filtering ratio, at most `300` spans
would be selected by such rule. This would effect in having `sampling.probability=0.06` (`300/5000=0.6`). If such value is already
set by head-based (or other) sampling, it's multiplied by the calculated value.

## Rejected trace configuration

It is possible to specify conditions for traces which should be fully dropped, without including them in probabilistic
filtering or additional policy evaluation. This typically happens e.g. when healthchecks are filtered-out.

Each of the specified drop rules has several properties:
- `name` (required): identifies the rule
- `name_pattern: <regex>`: selects the span if its operation name matches the provided regular expression
- `attributes: <list of attributes>`: list of attribute-level filters (both span level and resource level is being evaluated).  
  When several elements are specified, conditions for each of them must be met. Each entry might contain a number of fields:
  - `key: <name>`: name of the attribute key
  - `values: [<value1>, value2>]` (default=`empty`): list of string values, when present at least
    one of them must be matched
  - `use_regex: <use_regex>` (default=`false`): indication whether values provided should be treated as regular expressions
  - `ranges: [{min: <min_value>, max: <max_value>}]` (default=`empty`): list of numeric ranges; when present at least
    one must be matched


## Accepted trace configuration

Each defined policy is evaluated with order as specified in config. There are several properties:
- `name` (required): identifies the policy
- `spans_per_second` (default = 0): defines maximum number of spans per second that could be handled by this policy. When set to `-1`,
it selects the traces only if the global limit is not exceeded by other policies (however, without further limitations)

Additionally, each of the policy might have any of the following filtering criteria defined. They are evaluated for 
each of the trace spans. If at least one span matching all defined criteria is found, the trace is selected:
- `attributes: <list of attributes>`: list of attribute-level filters (both span level and resource level is being evaluated).  
When several elements are specified, conditions for each of them must be met. Each entry might contain a number of fields:
  - `key: <name>`: name of the attribute key
  - `values: [<value1>, value2>]` (default=`empty`): list of string values, when present at least 
  one of them must be matched
  - `use_regex: <use_regex>` (default=`false`): indication whether values provided should be treated as regular expressions
  - `ranges: [{min: <min_value>, max: <max_value>}]` (default=`empty`): list of numeric ranges; when present at least
  one must be matched
- `properties: { min_number_of_errors: <number>}`: selects the trace if it has at least provided number of errors 
(determined based on the span status field value)
- `properties: { min_number_of_spans: <number>}`: selects the trace if it has at least provided number of spans
- `properties: { min_duration: <duration>}`: selects the span if the duration is greater or equal the given value 
(use `s` or `ms` as the suffix to indicate unit)
- `properties: { name_pattern: <regex>`}: selects the span if its operation name matches the provided regular expression
- _(deprecated)_ `numeric_attribute: {key: <name>, min_value: <min_value>, max_value: <max_value>}`: selects span by matching numeric
  attribute (either at resource of span level)
- _(deprecated)_ `string_attribute: {key: <name>, values: [<value1>, <value2>], use_regex: <use_regex>}`: selects span by matching string attribute that is one
  of the provided values (either at resource of span level); when `use_regex` (`false` by default) is set to `true`
  the provided collection of values is evaluated as regular expressions

To invert the decision (which is still a subject to rate limiting), additional property can be configured:
- `invert_match: <invert>` (default=`false`): when set to `true`, the opposite decision is selected for the trace. E.g.
if trace matches a given string attribute and `invert_match=true`, then the trace is not selected

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

## Examples

### Just filtering out healthchecks

Following example will drop all traces that match either of the following criteria:
* there is a span which name starts with "health"
* there is a span coming from a service named "healthcheck"

```yaml
processors:
  cascading_filter:
    trace_reject_filters:
      - name: remove-all-traces-with-health-span
        name_pattern: "health.*"
      - name: remove-all-traces-with-healthcheck-service
        attributes: 
        - key: service.name
          values: 
           - "healthcheck/.*"
          use_regex: true
 ```

### Filtering out healhtchecks and traffic shaping

In the following example few more conditions were added:
* probabilistic filtering was set; it will randomly select traces for a total of up to 100 spans/second
* two traffic-shaping rules are applied:
  * traces which have minimum duration of 3s are selected (for up to 500 spans/second)
  * traces which have at least 3 error spans are selected (for up to 500 spans/second)

Basing on those rules, at most 1100 spans/second will be outputted.

```yaml
cascadingfilter:
  probabilistic_filtering_rate: 100
  trace_reject_filters:
    - name: remove-all-traces-with-health-span
      name_pattern: "health.*"
    - name: remove-all-traces-with-healthcheck-service
      attributes:
      - key: service.name
        values:
          - "healthcheck/.*"
        use_regex: true
  trace_accept_filters:
    - name: tail-based-duration
      properties:
        min_duration: 3s
      spans_per_second: 500 # <- adjust the output traffic level
    - name: tail-based-errors
      properties:
        min_number_of_errors: 3
      spans_per_second: 500 # <- adjust the output traffic level
```

### Advanced configuration 

It is additionally possible to use adaptive sampling, which will split the
total spans per second budget across all the rules evenly (for up to specified limit).
Additionally, it can be set that if there's any budget left, it can be filled with random traces.

```yaml
cascadingfilter:
  decision_wait: 30s
  num_traces: 200000
  expected_new_traces_per_sec: 2000
  spans_per_second: 1800
  probabilistic_filtering_rate: 100
  trace_reject_filters:
    - name: remove-all-traces-with-health-span
      name_pattern: "health.*"
    - name: remove-all-traces-with-healthcheck-service
      attributes:
        - key: service.name
          values:
            - "healthcheck/.*"
          use_regex: true
  trace_accept_filters:
    - name: tail-based-duration
      properties:
        min_duration: 3s
      spans_per_second: 500 # <- adjust the output traffic level
    - name: tail-based-errors
      properties:
        min_number_of_errors: 3
      spans_per_second: 500 # <- adjust the output traffic level
    - name: traces-with-foo-span-and-high-latency
      properties:
        name_pattern: "foo.*"
        min_duration: 10s
      spans_per_second: 1000 # <- adjust the output traffic level
    - name: some-service-traces-with-some-attribute
      attributes:
        - key: service.name
          values:
            - some-service
        - key: important-key
          values:
            - value1
            - value2
          use_regex: true
      spans_per_second: 300 # <- adjust the output traffic level
    - name: everything_else
      spans_per_second: -1 # If there's anything left in the budget, it will randomly select remaining traces
```

Refer to [cascading_filter_config.yaml](./testdata/cascading_filter_config.yaml) for detailed
examples on using the processor.
