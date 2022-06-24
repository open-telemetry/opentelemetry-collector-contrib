# Filter Processor

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | metrics, logs, traces |
| Distributions            | [core], [contrib]     |

The filter processor can be configured to include or exclude:

- logs, based on resource attributes using the `strict` or `regexp` match types
- metrics based on metric name in the case of the `strict` or `regexp` match types,
  or based on other metric attributes in the case of the `expr` match type.
  Please refer to [config.go](./config.go) for the config spec.
- Spans based on span names, and resource attributes, all with full regex support

It takes a pipeline type, of which `logs` `metrics`, and `traces` are supported, followed
by an action:

- `include`: Any names NOT matching filters are excluded from remainder of pipeline
- `exclude`: Any names matching filters are excluded from remainder of pipeline

For the actions the following parameters are required:

For logs:

- `match_type`: `strict`|`regexp`
- `resource_attributes`: ResourceAttributes defines a list of possible resource
  attributes to match logs against.
  A match occurs if any resource attribute matches all expressions in this given list.
- `record_attributes`: RecordAttributes defines a list of possible record
  attributes to match logs against.
  A match occurs if any record attribute matches all expressions in this given list.

For metrics:

- `match_type`: `strict`|`regexp`|`expr`
- `metric_names`: (only for a `match_type` of `strict` or `regexp`) list of strings
  or re2 regex patterns
- `expressions`: (only for a `match_type` of `expr`) list of expr expressions
  (see "Using an 'expr' match_type" below)
- `resource_attributes`: ResourceAttributes defines a list of possible resource
  attributes to match metrics against.
  A match occurs if any resource attribute matches all expressions in this given list.

This processor uses [re2 regex][re2_regex] for regex syntax.

[re2_regex]: https://github.com/google/re2/wiki/Syntax

More details can found at [include/exclude metrics](../attributesprocessor/README.md#includeexclude-filtering).

Examples:

```yaml
processors:
  filter/1:
    metrics:
      include:
        match_type: regexp
        metric_names:
          - prefix/.*
          - prefix_.*
        resource_attributes:
          - Key: container.name
            Value: app_container_1
      exclude:
        match_type: strict
        metric_names:
          - hello_world
          - hello/world
  filter/2:
    logs:
      include:
        match_type: strict
        resource_attributes:
          - Key: host.name
            Value: just_this_one_hostname
    logs/regexp:
        match_type: regexp
        resource_attributes:
          - Key: host.name
            Value: prefix.*
    logs/regexp_record:
        match_type: regexp
        record_attributes:
          - Key: record_attr
            Value: prefix_.*
```

Refer to the config files in [testdata](./testdata) for detailed
examples on using the processor.

## Using an 'expr' match_type

In addition to matching metric names with the 'strict' or 'regexp' match types, the filter processor
supports matching entire `Metric`s using the [expr](https://github.com/antonmedv/expr) expression engine.

The 'expr' filter evaluates the supplied boolean expressions _per datapoint_ on a metric, and returns a result
for the entire metric. If any datapoint evaluates to true then the entire metric evaluates to true, otherwise
false.

Made available to the expression environment are the following:

* `MetricName`
    a variable containing the current Metric's name
* `Label(name)`
    a function that takes a label name string as an argument and returns a string: the value of a label with that
    name if one exists, or ""
* `HasLabel(name)`
    a function that takes a label name string as an argument and returns a boolean: true if the datapoint has a label
    with that name, false otherwise

Example:

```yaml
processors:
  filter/1:
    metrics:
      exclude:
        match_type: expr
        expressions:
        - MetricName == "my.metric" && Label("my_label") == "abc123"
```

The above config will filter out any Metric that both has the name "my.metric" and has at least one datapoint
with a label of 'my_label="abc123"'.

### Support for multiple expressions

As with "strict" and "regexp", multiple "expr" `expressions` are allowed.

For example, the following two filters have the same effect: they filter out metrics named "system.cpu.time" and
"system.disk.io". 

```
processors:
  filter/expr:
    metrics:
      exclude:
        match_type: expr
        expressions:
          - MetricName == "system.cpu.time"
          - MetricName == "system.disk.io"
  filter/strict:
    metrics:
      exclude:
        match_type: strict
        metric_names:
          - system.cpu.time
          - system.disk.io
```

The expressions are effectively ORed per datapoint. So for the above 'expr' configuration, given a datapoint, if its
parent Metric's name is "system.cpu.time" or "system.disk.io" then there's a match. The conditions are tested against
all the datapoints in a Metric until there's a match, in which case the entire Metric is considered a match, and in
the above example the Metric will be excluded. If after testing all the datapoints in a Metric against all the
expressions there isn't a match, the entire Metric is considered to be not matching.


### Filter metrics using resource attributes
In addition to the names, metrics can be filtered using resource attributes. `resource_attributes` takes a list of resource attributes to filter metrics against. 

Following example will include only the metrics coming from `app_container_1` (the value for `container.name` resource attribute is `app_container_1`). 

```yaml
processors:
  filter:
    metrics:
      include:
        match_type: strict
        metric_names:
          - hello_world
          - hello/world
        resource_attributes:
          - Key: container.name
            Value: app_container_1
```

Following example will exclude all the metrics coming from `app_container_1` (the value for `container.name` resource attribute is `app_container_1`). 

```yaml
processors:
  filter:
    metrics:
      exclude:
        match_type: strict
        metric_names:
          - hello_world
          - hello/world
        resource_attributes:
          - Key: container.name
            Value: app_container_1
```

We can also use `regexp` to filter metrics using resource attributes. Following example will include only the metrics coming from `app_container_1` or `app_container_2` (the value for `container.name` resource attribute is either `app_container_1` or `app_container_2`). 

```yaml
processors:
  filter:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          - hello_world
          - hello/world
        resource_attributes:
          - Key: container.name
            Value: (app_container_1|app_container_1)
```

In case the no metric names are provided, `matric_names` being empty, the filtering is only done at resource level.

### Filter Spans from Traces

* This pipeline is able to drop spans and whole traces 
* Note: If this drops a parent span, it does not search out it's children leading to a missing Span in your trace visualization

See the documentation in the [attribute processor](../attributesprocessor/README.md) for syntax

For spans, one of Services, SpanNames, Attributes, Resources or Libraries must be specified with a
non-empty value for a valid configuration.

```yaml
processors:
  filter:
    spans:
      include:
        match_type: strict
        services:
          - app_3
      exclude:
        match_type: regex
        services:
          - app_1
          - app_2
        span_names:
          - hello_world
          - hello/world
        attributes:
          - Key: container.name
            Value: (app_container_1|app_container_2)
        libraries:
          - Name: opentelemetry
            Version: 0.0-beta
        resources:
          - Key: container.host
            Value: (localhost|127.0.0.1)
```

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol
