## `otelcol` operator

The `otelcol` operator parses the string-type field selected by `parse_from` as a collector self-log (a log line emitted by the collector's own zap logger), recognizing either of zap's `json` or `console` encodings, and promotes the nested `resource` object onto the entry's Resource.

### Configuration Fields

| Field        | Default          | Description |
| ---          | ---              | ---         |
| `id`         | `otelcol`        | A unique identifier for the operator. |
| `output`     | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from` | `body`           | The [field](../types/field.md) from which the value will be parsed. |
| `parse_to`   | `attributes`     | The [field](../types/field.md) to which the value will be parsed. |
| `on_error`   | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`         |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `format`     | `auto`           | One of `auto`, `json`, `console`. Forces a specific zap encoding instead of detecting it per line. |
| `timestamp`  | `nil`            | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`   | `nil`            | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator. |

### Embedded Operations

The `otelcol` operator can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

### Example Configurations

#### Parse a json-encoded self-log line

Configuration:
```yaml
- type: otelcol
```

<table>
<tr><td> Input body </td> <td> Output entry</td></tr>
<tr>
<td>

```json
{
  "body": "{\"ts\":\"2026-07-06T22:56:21.989Z\",\"level\":\"warn\",\"msg\":\"Failed to scrape Prometheus endpoint\",\"resource\":{\"k8s.pod.name\":\"otel-agent-qkvqj\",\"service.name\":\"otel-agent\"},\"otelcol.component.id\":\"receiver_creator\"}"
}
```

</td>
<td>

```json
{
  "timestamp": "2026-07-06T22:56:21.989Z",
  "severity": "warn",
  "body": "Failed to scrape Prometheus endpoint",
  "resource": {
    "k8s.pod.name": "otel-agent-qkvqj",
    "service.name": "otel-agent"
  },
  "attributes": {
    "otelcol.component.id": "receiver_creator"
  }
}
```

</td>
</tr>
</table>

#### Parse a console-encoded self-log line, auto-detected

Configuration:
```yaml
- type: otelcol
```

<table>
<tr><td> Input body </td> <td> Output entry</td></tr>
<tr>
<td>

```json
{
  "body": "2026-07-06T22:56:21.989Z\twarn\tinternal/transaction.go:127\tFailed to scrape Prometheus endpoint\t{\"resource\":{\"k8s.pod.name\":\"otel-agent-qkvqj\"},\"otelcol.component.id\":\"receiver_creator\"}"
}
```

</td>
<td>

```json
{
  "timestamp": "2026-07-06T22:56:21.989Z",
  "severity": "warn",
  "body": "Failed to scrape Prometheus endpoint",
  "resource": {
    "k8s.pod.name": "otel-agent-qkvqj"
  },
  "attributes": {
    "caller": "internal/transaction.go:127",
    "otelcol.component.id": "receiver_creator"
  }
}
```

</td>
</tr>
</table>

#### Force a specific encoding, skipping auto-detection

Configuration:
```yaml
- type: otelcol
  format: json
```

Use this when every line in the file is known to use one encoding, since `format: auto` (the default) inspects each line individually to decide which parser to apply.

#### Only parse lines from the collector's own log file

Configuration:
```yaml
- type: otelcol
  if: 'attributes["log.file.name"] == "collector.log"'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "attributes": {
    "log.file.name": "collector.log"
  },
  "body": "{\"ts\":\"2026-07-06T22:56:21.989Z\",\"level\":\"info\",\"msg\":\"Starting GRPC server\"}"
}
```

</td>
<td>

```json
{
  "attributes": {
    "log.file.name": "collector.log"
  },
  "timestamp": "2026-07-06T22:56:21.989Z",
  "severity": "info",
  "body": "Starting GRPC server"
}
```

</td>
</tr>

<tr>
<td>

```json
{
  "attributes": {
    "log.file.name": "app.log"
  },
  "body": "{\"ts\":\"2026-07-06T22:56:21.989Z\",\"level\":\"info\",\"msg\":\"Starting GRPC server\"}"
}
```

</td>
<td>

```json
{
  "attributes": {
    "log.file.name": "app.log"
  },
  "body": "{\"ts\":\"2026-07-06T22:56:21.989Z\",\"level\":\"info\",\"msg\":\"Starting GRPC server\"}"
}
```

</td>
</tr>
</table>