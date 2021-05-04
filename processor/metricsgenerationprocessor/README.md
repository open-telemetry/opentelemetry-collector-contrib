# Metrics Generation Processor
**Status: under development; Not recommended for production usage.**

Supported pipeline types: metrics

## Description

The metrics generation processor (`metricsgenerationprocessor`) can be used to create new metrics using existing metrics following a given rule. Currently it supports following two approaches for creating a new metric.

1. It can create a new metric from two existing metrics by applying one of the folliwing arithmetic operations: add, subtract, multiply, divide and percent. One use case is to calculate the `pod.memory.utilization` metric like the following equation-
`pod.memory.utilization` = (`pod.memory.usage.bytes` / `node.memory.limit`)
1. It can create a new metric by scaling the value of an existing metric with a given constant number. One use case is to convert `pod.memory.usage` metric values from Megabytes to Bytes (multiply the existing metric's value by 1000000)

## Configuration

Configuration is specified through a list of generation rules. Generation rules find the metrics which 
matche the given metric names apply the operation to those metrics.

```yaml
processors:
    # processor name: metricsgeneration
    metricsgeneration:

        # specify the metric generation rules
        generation_rules:
              # name of the new metric. this is a required field
            - new_metric_name: <new_metric_name>

              # generation_type describes how the metric will be generated. it can either be calculate or scale calculate generates a metric applying the given operation on two operand metrics. scale operates only on  operand1 metric to generate the new metric.
              generation_type: {calculate, scale}

              # this is a required field
              operand1_metric: <first_operand_metric>

              # this field is required only if the generation_type is calculate
              operand2_metric: <second_operand_metric>

              # operation specifies which atrithmatic operation to apply. it can be one of the five supported operations.
              operation: {add, subtract, multiply, divide, percent}
```

## Example Configurations

### Create a new metric using two existing metrics
```yaml
# create pod.cpu.utilized following (pod.cpu.usage / node.cpu.limit)
generation_rules:
    - new_metric_name: pod.cpu.utilized
      generation_type: calculate
      operand1_metric: pod.cpu.usage
      operand2_metric: node.cpu.limit
      operation: divide
```

### Create a new metric scaling the value of an existing metric
```yaml
# create pod.memory.usage.bytes from pod.memory.usage.megabytes
generation_rules:
    - new_metric_name: pod.memory.usage.bytes
      generation_type: scale
      operand1_metric: pod.memory.usage.megabytes
      operation: multiply
      scale_by: 1000000
```
