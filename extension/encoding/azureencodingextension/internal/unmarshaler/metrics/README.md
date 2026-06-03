# Metrics transformation rules

Supported Time Grains: PT1M, PT5M, PT15M, PT30M, PT1H, PT6H, PT12H, P1D

## General transformation rules

By default, 1 incoming metric point (with specific `metricName`) from Azure will be transformed into 5 OpenTelemetry metrics:

* `metricName_total` (type - Gauge)
* `metricName_count` (type - Gauge)
* `metricName_minimum` (type - Gauge)
* `metricName_maximum` (type - Gauge)
* `metricName_average` (type - Gauge)

`start_time_unix_nano` is calculated as `time - timeGrain`, where `timeGrain` can be one of supported Time Grains, converted to time delta (e.g. `PT15M` is 15 minutes delta).

| Azure      | OpenTelemetry            | OpenTelemetry Scope |
|------------|--------------------------|---------------------|
| time       | time_unix_nano           | Metric DataPoint    |
| resourceId | cloud.resource_id        | Resource Attributes |
| metricName | name                     | Metric              |
| timeGrain  | start_time_unix_nano     | Metric DataPoint    |
| total      | metricName + "_total"    | Metric DataPoint    |
| count      | metricName + "_count"    | Metric DataPoint    |
| minimum    | metricName + "_minimum"  | Metric DataPoint    |
| maximum    | metricName + "_maximum"  | Metric DataPoint    |
| average    | metricName + "_average"  | Metric DataPoint    |
