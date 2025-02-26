# Translation Rules Migration Guide

## Context

The `translation_rules` configuration option of the SignalFx exporter
[has been deprecated](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/18218)
and [removed](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/35332) in an effort to
reduce the exporter's complexity and leverage existing functionality in existing
OpenTelemetry Collector processors.

## Migration Guide

The following table can be referenced to map existing translation rule actions to processors that
can be used to accomplish the same functionality.

| Deleted Translation rule | Replacement option | Replacement example |
  | -----------------|--------------------|----------------------|
| aggregate_metric | `transform` processor's `aggregate_on_attributes` function with the `metric` context | [aggregate example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#aggregate_on_attributes) |
| calculate_new_metric | `metricsgeneration` processor's `calculate` functionality | [calculate example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricsgenerationprocessor#example-configurations) |
| convert_values | `transform` processor's `Double` or `Int` converter on a `datapoint` context | [`Double` example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#double), [`Int` example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#int) |
| copy_metrics | `metricstransform` processor's `insert` functionality | [copy all datapoints](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#create-a-new-metric-from-an-existing-metric), [conditionally copy datapoints](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#create-a-new-metric-from-an-existing-metric-with-matching-label-values) |
| delta_metric | `cumulativetodelta` processor. To preserve original metrics, first copy the original metric, then use the copied metric in the `cumulativetodelta` processor | [specify which metrics to convert example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#examples)
| drop_dimensions | `transform` processor's `delete_keys` function with the `datapoint` context | [simple example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#delete_key), use a `where` clause with the given example to filter based upon the metric name or dimension value |
| drop_metrics | `filter` processor | [drop by name and value example](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor#dropping-specific-metric-and-value) |
| multiply_int, divide_int, multiply_float | `metricstransform` processor's `scale` value functionality | [one metric](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#scale-value) |
| rename_dimension_keys | `metricstransform` processor's update label function | [one metric](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#rename-labels), [multiple metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#rename-labels-for-multiple-metrics) |
| rename_metrics | `metricstransform` processor's rename metric functionality | [one metric](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#rename-metric), [multiple metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#rename-multiple-metrics-using-substitution) |
| split_metric | `metricstransform` processor's `insert` functionality and `filter` processor | Refer to the replacement guidance for the `copy_metrics` and `drop_metrics` translation rules |