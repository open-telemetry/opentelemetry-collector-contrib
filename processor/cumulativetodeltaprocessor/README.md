# Cumulative to Delta Processor

| Status                   |                           |
|--------------------------|---------------------------|
| Stability                | [beta]                    |
| Supported pipeline types | metrics                   |
| Distributions            | [contrib]                 |
| Warnings                 | [Statefulness](#warnings) |

## Description

The cumulative to delta processor (`cumulativetodeltaprocessor`) converts monotonic, cumulative sum and histogram metrics to monotonic, delta metrics. Non-monotonic sums and exponential histograms are excluded.

Histogram conversion is currently behind a [feature gate](#feature-gate-configurations) and will only be converted if the feature flag is set.

## Configuration

Configuration is specified through a list of metrics. The processor uses metric names to identify a set of cumulative metrics and converts them from cumulative to delta.

The following settings can be optionally configured:

- `include`: List of metrics names or patterns to convert to delta.
- `exclude`: List of metrics names or patterns to not convert to delta.  **If a metric name matches both include and exclude, exclude takes precedence.**
- `max_stale`: The total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely. Default: 0

If neither include nor exclude are supplied, no filtering is applied.

#### Examples

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # list the exact cumulative sum or histogram metrics to convert to delta
        include:
            metrics:
                - <metric_1_name>
                - <metric_2_name>
                .
                .
                - <metric_n_name>
            match_type: strict
```

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # Convert cumulative sum or histogram metrics to delta
        # if and only if 'metric' is in the name
        include:
            metrics:
                - "*metric*"
            match_type: regexp
```

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # Convert cumulative sum or histogram metrics to delta
        # if and only if 'metric' is not in the name
        exclude:
            metrics:
                - "*metric*"
            match_type: regexp
```

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:
        # If include/exclude are not specified
        # convert all cumulative sum or histogram metrics to delta
```

## Feature gate configurations

The **processor.cumulativetodeltaprocessor.EnableHistogramSupport** feature flag controls whether cumulative histograms delta conversion is supported or not. It is disabled by default, meaning histograms will not be modified by the processor.  If enabled, which histograms are converted is still subjected to the processor's include/exclude filtering.

Pass `--feature-gates processor.cumulativetodeltaprocessor.EnableHistogramSupport` to enable this feature.

This feature flag will be removed, and histograms will be enabled by default in release v0.60.0, September 2022.

## Warnings

- [Statefulness](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#statefulness): The cumulativetodelta processor's calculates delta by remembering the previous value of a metric.  For this reason, the calculation is only accurate if the metric is continuously sent to the same instance of the collector.  As a result, the cumulativetodelta processor may not work as expected if used in a deployment of multiple collectors.  When using this processor it is best for the data source to being sending data to a single collector.


[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
