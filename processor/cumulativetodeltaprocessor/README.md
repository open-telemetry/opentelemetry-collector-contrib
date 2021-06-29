# Cumulative to Delta Processor
**Status: under development; Not recommended for production usage.**

Supported pipeline types: metrics

## Description

The cumulative to delta processor (`cumulativetodeltaprocessor`) converts cumulative sum metrics to cumulative delta. 

## Configuration

Configuration is specified through a list of metrics. The processor uses metric names to identify a set of cumulative sum metrics and converts them to cumulative delta.

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # list the cumulative sum metrics to convert to delta
        metrics:
            - <metric_1_name>
            - <metric_2_name>
            .
            .
            - <metric_n_name>
```