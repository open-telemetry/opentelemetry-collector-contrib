# Cumulative to Delta Processor
**Status: under development; Not recommended for production usage.**

Supported pipeline types: metrics

## Description

The cumulative to delta processor (`cumulativetodeltaprocessor`) converts cumulative sum metrics to cumulative delta. 

## Configuration

Configuration is specified through a list of metrics. The processor uses metric name as well as resource attributes and labels to uniquely identify a metric and converts it to cumulative delta.

```yaml
processors:
    # processor name: cumulativetodelta
    cumulativetodelta:

        # list the cumulative sum metrics to convert to delta
        metrics:
              # Name of the metric. This is a required field.
            - name: <metric_name>

              # List of resource attribte keys against which the metric will be matched
              resource_attribute_keys: [attribute1, attribute2]

              # List of metric label keys against which the metric will be matched
              metric_label_keys: [label1, label2]
```