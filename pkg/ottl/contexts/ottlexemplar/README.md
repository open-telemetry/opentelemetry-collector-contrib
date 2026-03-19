# OTTL Exemplar Context

The `exemplar` context allows OTTL statements to read and modify individual
[exemplars](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exemplars)
attached to metric datapoints.

## Paths

The following paths are supported. All paths also support access to parent
contexts via their context prefix (e.g. `resource.attributes`, `metric.name`).

| Path | Type | Description |
|---|---|---|
| `exemplar.time_unix_nano` | `int64` | Exemplar timestamp as Unix nanoseconds |
| `exemplar.time` | `time.Time` | Exemplar timestamp as `time.Time` |
| `exemplar.double_value` | `float64` | Double observation value |
| `exemplar.int_value` | `int64` | Integer observation value |
| `exemplar.trace_id` | `pcommon.TraceID` | Associated trace ID |
| `exemplar.span_id` | `pcommon.SpanID` | Associated span ID |
| `exemplar.filtered_attributes` | `pcommon.Map` | Attributes filtered from the original measurement |
| `exemplar.filtered_attributes["key"]` | any | Individual filtered attribute value |
| `exemplar.cache` | `pcommon.Map` | Mutable per-exemplar cache (not exported) |

## Example: shift exemplar timestamps by a fixed offset

```yaml
processors:
  transform/shift_timestamps:
    error_mode: ignore
    metric_statements:
      - context: exemplar
        statements:
          - set(exemplar.time, exemplar.time + Duration("2h")) where exemplar.time_unix_nano != 0
```
