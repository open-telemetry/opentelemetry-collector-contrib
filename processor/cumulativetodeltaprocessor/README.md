# Cumulative to Delta Processor
**Status: under development; Not recommended for production usage.**

Supported pipeline types: metrics

## Description

The cumulative to delta processor (`cumulativetodeltaprocessor`) converts cumulative sum metrics to cumulative delta. 

## Configuration

The default configuration is to convert all monotonic sum metrics from aggregation temporality cumulative to aggregation temporality delta.

The following settings can be optionally configured:

- `metrics`: The processor uses metric names to identify a set of cumulative sum metrics and converts them to cumulative delta. Defaults to converting all metric names.
- `max_stale`: The total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely. Default: 0

#### Example

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # list the cumulative sum metrics to convert to delta
        # (optional - defaults to converting all monotonic cumulative sum metrics)
        metrics:
            - <metric_1_name>
            - <metric_2_name>
            .
            .
            - <metric_n_name>
```
