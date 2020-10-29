# Metrics Transform Processor

Supported pipeline types: metrics

## Description

The metrics transform processor can be used to rename metrics, and add, rename or delete label keys and values. It can also be used to perform aggregations on metrics across labels or label values. The complete list of supported operations that can be applied to one or more metrics is provided in the below table.

:information_source: This processor only supports renames/aggregations **within a batch of metrics**. It does not do any aggregation across batches, so it is not suitable for aggregating metrics from multiple sources (e.g. multiple nodes or clients).

| Operation | Example |
| --- | --- |
| Rename metrics | Rename `system.cpu.usage` to `system.cpu.usage_time` |
| Add labels | For metric `system.cpu.usage`, add new label `identifier` with value `1` to all data points |
| Rename label keys | For metric `system.cpu.usage`, rename label `state` to `cpu_state` |
| Rename label values | For metric `system.cpu.usage`, label `state`, rename label value `idle` to `-` |
| Delete label values | For metric `system.cpu.usage`, delete all data points where label `state` has value `idle` |
| Toggle the data type of scalar metrics between `int` and `double` | For metric `system.cpu.usage`, change from `int` data points to `double` data points |
| Aggregate across label sets by sum, mean, min, or max | For metric `system.cpu.usage`, retain the label `state`, but aggregate away the label `cpu` |
| Aggregate across label values by sum, mean, min, or max | For metric `system.cpu.usage`, label `state`, calculate `used = sum{user + system}` |

In addition to the above, when adding or updating a label value, specify `{{version}}` to include the application version number

## Configuration

Configuration is specified through a list of transformations and operations. Transformations and operations will be applied to all metrics in order so that later transformations or operations may reference the result of previous transformations or operations.

```yaml
# transforms is a list of transformations with each element transforming a metric selected by metric name
transforms:
    # include specifies the metric name used to determine which metric(s) to operate on
  - include: <metric_name>
    # match_type specifies whether the include name should be used as a strict match or regexp match, default = strict
    match_type: {strict, regexp}
    # action specifies if the operations are performed on the metric in place, or on an inserted clone
    action: {update, insert}
    # new_name specifies the updated name of the metric; if action is insert, new_name is required
    new_name: <new_metric_name_inserted>
    # operations contain a list of operations that will be performed on the selected metrics
    operations:
        # update_label action can be used to update the name of a label or the values of this label
      - action: {add_label, update_label, delete_label_value, toggle_scalar_data_type, aggregate_labels, aggregate_label_values}
        # label specifies the label to operate on
        label: <label>
        # new_label specifies the updated name of the label; if action is add_label, new_label is required
        new_label: <new_label>
        # aggregated_values contains a list of label values that will be aggregated; if action is aggregate_label_values, aggregated_values is required
        aggregated_values: [values...]
        # new_label specifies the updated name of the label value; if action is add_label or aggregate_label_values, new_value is required
        new_value: <new_value>
        # label_set contains a list of labels that will remain after aggregation; if action is aggregate_labels, label_set is required
        label_set: [labels...]
        # aggregation_type defines how excluded labels will be aggregated
        aggregation_type: {sum, mean, max}
        # value_actions contain a list of operations that will be performed on the selected label
        value_actions:
            # value specifies the value to operate on
          - value: <current_label_value>
            # new_value specifies the updated value
            new_value: <new_label_value>
```

## Examples

### Create a new metric from an existing metric
```yaml
# create host.cpu.utilization from host.cpu.usage
include: host.cpu.usage
action: insert
new_name: host.cpu.utilization
operations:
  ...
```

### Rename metric
```yaml
# rename system.cpu.usage to system.cpu.usage_time
include: system.cpu.usage
action: update
new_name: system.cpu.usage_time
```

### Add a label
```yaml
# for system.cpu.usage_time, add label `version` with value `opentelemetry collector vX.Y.Z` to all points
include: system.cpu.usage
action: update
operations:
  - action: add_label
    new_label: version
    new_value: opentelemetry collector {{version}}
```

### Add a label to multiple metrics
```yaml
# for all system metrics, add label `version` with value `opentelemetry collector vX.Y.Z` to all points
include: ^system\.
match_type: regexp
action: update
operations:
  - action: add_label
    new_label: version
    new_value: opentelemetry collector {{version}}
```

### Rename labels
```yaml
# for system.cpu.usage_time, rename the label state to cpu_state
include: system.cpu.usage
action: update
operations:
  - action: update_label
    label: state
    new_label: cpu_state
```

### Rename labels for multiple metrics
```yaml
# for all system.cpu metrics, rename the label state to cpu_state
include: ^system\.cpu\.
action: update
operations:
  - action: update_label
    label: state
    new_label: cpu_state
```

### Rename label values
```yaml
# rename the label value slab_reclaimable to sreclaimable, slab_unreclaimable to sunreclaimable
...
operations:
  - action: update_label
    label: state
    value_actions:
      - value: slab_reclaimable
        new_value: sreclaimable
      - value: slab_unreclaimable
        new_value: sunreclaimable
```

### Delete label value
```yaml
# delete the label value 'value' of the label 'label'
...
operation:
  - action: delete_label_value
    label: label
    label_value: value
```

### Toggle datatype
```yaml
# toggle the datatype for a metric
...
operation:
  - action: toggle_scalar_data_type
```

### Aggregate labels
```yaml
# aggregate away all labels except `state` using summation
...
operations:
  - action: aggregate_labels
    label_set: [ state ]
    aggregation_type: sum
```

### Aggregate label values
```yaml
# aggregate data points with state label value slab_reclaimable & slab_unreclaimable using summation
...
operations:
  - action: aggregate_label_values
    label: state
    aggregated_values: [ slab_reclaimable, slab_unreclaimable ]
    new_value: slab 
    aggregation_type: sum
```
