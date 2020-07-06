# Metrics Transform Processor **(UNDER DEVELOPMENT - NOT READY FOR USE)**
Supported pipeline types: metrics
- This ONLY supports renames/aggregations **within individual metrics**. It does not do any aggregation across batches, so it is not suitable for aggregating metrics from multiple sources (e.g. multiple nodes or clients). At this point, it is only for aggregating metrics from a single source that groups its metrics for a particular time period into a single batch (e.g. host metrics from the VM the collector is running on).
- Rename Collisions will result in a no operation on the metrics data
  - e.g. If want to rename a metric or label to `new_name` while there is already a metric or label called `new_name`, this operation will not take any effect. There will also be an error logged

## Description
The metrics transform processor can be used to rename metrics, labels, or label values. It can also be used to perform aggregations on metrics across labels or label values.

## Capabilities
- Rename metrics (e.g. rename `cpu/usage` to `cpu/usage_time`)
- Rename labels (e.g. rename `cpu` to `core`)
- Rename label values (e.g. rename `done` to `complete`)
- Aggregate across label sets (e.g. only want the label `usage`, but don’t care about the labels `core`, and `cpu`)
  - Aggregation_type: sum, average, max
- Aggregate across label values (e.g. want `memory{slab}`, but don’t care about `memory{slab_reclaimable}` & `memory{slab_unreclaimable}`)
  - Aggregation_type: sum, average, max

## Configuration
```yaml
# transforms is a list of transformations with each element transforming a metric selected by metric name
transforms:
  # name is used to match with the metric to operate on. This implementation doesn’t utilize the filtermetric’s MatchProperties struct because it doesn’t match well with what I need at this phase. All is needed for this processor at this stage is a single name string that can be used to match with selected metrics. The list of metric names and the match type in the filtermetric’s MatchProperties struct are unnecessary. Also, based on the issue about improving filtering configuration, it seems like this struct is subject to be slightly modified.
  - metric_name: <current_metric_name>

  # action specifies if the operations are performed on the current copy of the metric or on a newly created metric that will be inserted
    action: {update, insert}

  # new_name is used to rename metrics (e.g. rename cpu/usage to cpu/usage_time) if action is insert, new_name is required
    new_name: <new_metric_name_inserted>

  # operations contain a list of operations that will be performed on the selected metrics. Each operation block is a key-value pair, where the key can be any arbitrary string set by the users for readability, and the value is a struct with fields required for operations. The action field is important for the processor to identify exactly which operation to perform 
    operations:

    # update_label action can be used to update the name of a label or the values of this label (e.g. rename label `cpu` to `core`)
    - action: update_label
      label: <current_label1>
      new_label: <new_label>
      value_actions:
      - value: <current_label_value>
        new_value: <new_label_value>

    # aggregate_labels action aggregates metrics across labels (e.g. only want the label `usage`, but don’t care about the labels `core`, and `cpu`)
    - action: aggregate_labels
    # label_set contains a list of labels that will remain after the aggregation. The excluded labels will be aggregated by the way specified by aggregation_type.
      label_set: [labels...]
      aggregation_type: {sum, average, max}

    # aggregate_label_values action aggregates labels across label values (e.g. want memory{slab}, but don’t care about memory{slab_reclaimable} & memory{slab_unreclaimable})
    - action: aggregate_label_values
      label: <label>
    # aggregated_values contains a list of label values that will be aggregated by the way specified by aggregation_type into new_value. The excluded label values will remain.
      aggregated_values: [values...]
      new_value: <new_value> 
      aggregation_type: {sum, average, max}
```

## Examples

### Insert New Metric
```yaml
# create host.cpu.utilization from host.cpu.usage
metric_name: host/cpu/usage
action: insert
new_name: host/cpu/utilization
operations:
  ...
```

### Rename Labels
```yaml
# rename the label cpu to core
operations:
  - action: update_label
    label: cpu
    new_label: core
```

### Aggregate Labels
```yaml
# aggregate away everything but `state` using summation
...
operations:
  -action: aggregate_labels
   label_set: [ state ]
   aggregation_type: sum
```

### Aggregate Label Values
```yaml
# combine slab_reclaimable & slab_unreclaimable by summation
...
operations:
  -action: aggregate_label_values
   label: state
   aggregated_values: [ slab_reclaimable, slab_unreclaimable ]
   new_value: slab 
   aggregation_type: sum
```
