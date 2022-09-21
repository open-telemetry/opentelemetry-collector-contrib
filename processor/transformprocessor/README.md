# Transform Processor

| Status                   |                                                                                    |
|--------------------------|------------------------------------------------------------------------------------|
| Stability                | [alpha]                                                                            |
| Supported pipeline types | traces, metrics, logs                                                              |
| Distributions            | [contrib]                                                                          |
| Warnings                 | [Unsound Transformations, Identity Conflict, Orphaned Telemetry, Other](#warnings) |

The transform processor modifies telemetry based on configuration using the [OpenTelemetry Transformation Language](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/ottl).
The processor takes a list of queries for each signal type and executes the queries against the incoming telemetry in the order specified in the config.  Each query can access and transform telemetry using functions and allow the use of a condition to help decide whether the function should be executed.

## Config

The transform processor allows configuring queries for traces, metrics, and logs. Each signal specifies a list of string queries that get passed to the OTTL for interpretation.

```yaml
transform:
  <traces|metrics|logs>:
    queries:
      - string
      - string
      - string
```

## Example

Example configuration:
```yaml
transform:
  traces:
    queries:
      - set(status.code, 1) where attributes["http.path"] == "/health"
      - keep_keys(resource.attributes, "service.name", "service.namespace", "cloud.region", "process.command_line")
      - set(name, attributes["http.route"])
      - replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")
      - replace_pattern(resource.attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")
      - limit(attributes, 100)
      - limit(resource.attributes, 100)
      - truncate_all(attributes, 4096)
      - truncate_all(resource.attributes, 4096)
  metrics:
    queries:
      - set(metric.description, "Sum") where metric.type == "Sum"
      - keep_keys(resource.attributes, "host.name")
      - limit(attributes, 100, "host.name")
      - truncate_all(attributes, 4096)
      - truncate_all(resource.attributes, 4096)
      - convert_sum_to_gauge() where metric.name == "system.processes.count"
      - convert_gauge_to_sum("cumulative", false) where metric.name == "prometheus_metric"
  logs:
    queries:
      - set(severity_text, "FAIL") where body == "request failed"
      - replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")
      - replace_all_patterns(attributes, "/account/\\d{4}", "/account/{accountId}")
      - set(body, attributes["http.route"])
      - keep_keys(resource.attributes, "service.name", "service.namespace", "cloud.region")
```
## Grammar

You can learn more in-depth details on the capabilities and limitations of the OpenTelemetry Transformation Language used by the transform processor by reading about its [grammar](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/ottl#grammar).

## Contexts

The transform processor utilizes the OTTL's standard contexts for Traces, Metrics and Logs.  The contexts allow the OTTL to interact with the underlying telemetry data in its pdata form.

- [Traces Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/contexts/ottltraces)
- [Metrics Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/contexts/ottlmetrics)
- [Logs Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/contexts/ottllogs)

## Supported functions:

Since the transform processor utilizes the OTTL's contexts for Traces, Metrics, and Logs, it is able to utilize functions that expect pdata in addition to any common functions. These common functions can be used for any signal.

- [Common OTTL functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/functions/ottlcommon)
- [Otel OTTL functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/oteltransformationlanguage/functions/ottlotel)

In addition to OTTL functions, the processor defines its own functions to help with transformations specific to this processor:

**Metrics only functions**
- [convert_sum_to_gauge](#convert_sum_to_gauge)
- [convert_gauge_to_sum](#convert_gauge_to_sum)
- [convert_summary_count_val_to_sum](#convert_summary_count_val_to_sum)
- [convert_summary_sum_val_to_sum](#convert_summary_sum_val_to_sum)

## convert_sum_to_gauge

`convert_sum_to_gauge()`

Converts incoming metrics of type "Sum" to type "Gauge", retaining the metric's datapoints. Noop for metrics that are not of type "Sum".

**NOTE:** This function may cause a metric to break semantics for [Gauge metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#gauge). Use at your own risk.

Examples:

- `convert_sum_to_gauge()`

## convert_gauge_to_sum

`convert_gauge_to_sum(aggregation_temporality, is_monotonic)`

Converts incoming metrics of type "Gauge" to type "Sum", retaining the metric's datapoints and setting its aggregation temporality and monotonicity accordingly. Noop for metrics that are not of type "Gauge".

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) that specifies the resultant metric's aggregation temporality. `is_monotonic` is a boolean that specifies the resultant metric's monotonicity. 

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_gauge_to_sum("cumulative", false)`


- `convert_gauge_to_sum("delta", true)`

## convert_summary_count_val_to_sum

`convert_summary_count_val_to_sum(aggregation_temporality, is_monotonic)`

The `convert_summary_count_val_to_sum` function creates a new Sum metric from a Summary's count value.

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) representing the desired aggregation temporality of the new metric. `is_monotonic` is a boolean representing the monotonicity of the new metric.

The name for the new metric will be `<summary metric name>_count`. The fields that are copied are: `timestamp`, `starttimestamp`, `attibutes`, and `description`. The new metric that is created will be passed to all functions in the metrics queries list.  Function conditions will apply.

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_summary_count_val_to_sum("delta", true)`


- `convert_summary_count_val_to_sum("cumulative", false)`

## convert_summary_sum_val_to_sum

`convert_summary_sum_val_to_sum(aggregation_temporality, is_monotonic)`

The `convert_summary_sum_val_to_sum` function creates a new Sum metric from a Summary's sum value.

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) representing the desired aggregation temporality of the new metric. `is_monotonic` is a boolean representing the monotonicity of the new metric.

The name for the new metric will be `<summary metric name>_sum`. The fields that are copied are: `timestamp`, `starttimestamp`, `attibutes`, and `description`. The new metric that is created will be passed to all functions in the metrics queries list.  Function conditions will apply.

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_summary_sum_val_to_sum("delta", true)`


- `convert_summary_sum_val_to_sum("cumulative", false)`

## Contributing

See [CONTRIBUTING.md](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/CONTRIBUTING.md).


## Warnings

The transform processor's implementation of the [OpenTelemetry Transformation Language](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/processing.md#telemetry-query-language) (OTTL) allows users to modify all aspects of their telemetry.  Some specific risks are listed below, but this is not an exhaustive list.  In general, understand your data before using the transform processor.  

- [Unsound Transformations](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#unsound-transformations): Several Metric-only functions allow you to transform one metric data type to another or create new metrics from an existing metrics.  Transformations between metric data types are not defined in the [metrics data model](https://github.com/open-telemetry/opentelemetry-specification/blob/main//specification/metrics/data-model.md).  These functions have the expectation that you understand the incoming data and know that it can be meaningfully converted to a new metric data type or can meaningfully be used to create new metrics.
  - Although the OTTL allows the `set` function to be used with `metric.data_type`, its implementation in the transform processor is NOOP.  To modify a data type you must use a function specific to that purpose.
- [Identity Conflict](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#identity-conflict): Transformation of metrics have the potential to affect the identity of a metric leading to an Identity Crisis. Be especially cautious when transforming metric name and when reducing/changing existing attributes.  Adding new attributes is safe.
- [Orphaned Telemetry](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#orphaned-telemetry): The processor allows you to modify `span_id`, `trace_id`, and `parent_span_id` for traces and `span_id`, and `trace_id` logs.  Modifying these fields could lead to orphaned spans or logs. 
- The `limit` function drops attributes at random. If there are attributes that should never be dropped please provide them as function arguments, e.g. `limit(attibutes, 10, "http.host", "http.method")`

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
