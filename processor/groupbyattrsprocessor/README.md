# Group by Attributes processor

Supported pipeline types: traces, logs

This processor groups the records by provided attributes, extracting them from the 
record to resource level. When the grouped attribute key already exists at the resource-level,
it's value is being overwritten with the record-level one. The processor also merges collections of records 
under matching InstrumentationLibrary.

Typical use-cases:

* extracting resources from "flat" data formats, such as Fluentbit logs
* optimizing data packaging by extracting common attributes

Please refer to [config.go](./config.go) for the config spec.

Examples:

```yaml
processors:
  groupbyattrs:
    keys:
      - foo
      - bar
```

## Configuration

Refer to [config.yaml](./testdata/config.yaml) for detailed examples on using the processor.

The `keys` property describes which attribute keys should be considered for grouping, if any of them is found
the grouping occurs.

## Metrics

The following metrics are recorded by this processor:
* `num_grouped_spans` represents the number of spans that had attributes grouped
* `num_non_grouped_spans` represents the number of spans that did not have attributes grouped
* `span_groups` represents the distributon of groups extracted for spans
* `num_grouped_logs` represents the number of logs that had attributes grouped
* `num_non_grouped_logs` represents the number of logs that did not have attributes grouped
* `log_groups` represents the distributon of groups extracted for logs
