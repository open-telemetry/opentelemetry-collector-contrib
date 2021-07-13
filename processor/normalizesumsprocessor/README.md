# Normalize Sums Processor

Supported pipeline types: metrics

This is a processor for supporting normalization from sum metrics
where an appropriate start time is not available to a sum metric using
the first data point as the start. The first metric of this type will not 
be emitted, and future metrics will be transformed into a sum metric
normalized to treat the first data point (or a subsequent data point
where a reset occurred) as the start.

Additionally, any time a metric value decreases, it will be assumed to
have rolled over, at which point another data point will be skipped in
favor of providing only accurate data representing the sum from a known point

## Configuration

List the metrics being transformed under 'transforms'. If any need to be renamed,
a new name can be provided, the old metric will be removed.

```yaml
normalizesums:
  transforms:
    - metric_name: <metric_name>
      new_name: <new_metric_name> # optional. If not provided, transforms in place
    - metric_name: <another_metric_name>
```


If you do not provide a transforms list, any Sum type metrics found will be transformed

```yaml
normalizesums:
```
