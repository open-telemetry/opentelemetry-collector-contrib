# Cumulative to Delta Processor

| Status                   |           |
| ------------------------ | --------- |
| Stability                | [beta]    |
| Supported pipeline types | metrics   |
| Distribution             | [contrib] |

## Description

The cumulative to delta processor (`cumulativetodeltaprocessor`) converts monotonic, cumulative sum metrics to monotonic, delta sum metrics. Non-monotonic sums are excluded.

## Configuration

Configuration is specified through a list of metrics. The processor uses metric names to identify a set of cumulative metrics and converts them from cumulative to delta.

The following settings can be optionally configured:

- `include`: List of metrics names or patterns to convert to delta.
- `exclude`: List of metrics names or patterns to not convert to delta.  **If a metric name matches both include and exclude, exclude takes precedence.**
- `max_stale`: The total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely. Default: 0
- `metrics`: Deprecated. The processor uses metric names to identify a set of cumulative metrics and converts them to delta.

If neither include nor exclude are supplied, no filtering is applied.

#### Examples

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # list the exact cumulative sum metrics to convert to delta
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

        # Convert cumulative sum metrics to delta 
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

        # Convert cumulative sum metrics to delta 
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
        # convert all cumulative sum metrics to delta
```

[beta]: https://github.com/open-telemetry/opentelemetry-collector-contrib#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
