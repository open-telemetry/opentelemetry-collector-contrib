## `otelcol` operator

The `otelcol` operator parses the entry body as a collector self-log (a log line emitted by the collector's own zap logger), recognizing either of zap's `json` or `console` encodings, and promotes the nested `resource` object onto the entry's Resource.

### Configuration Fields

| Field        | Default          | Description |
| ---          | ---              | ---         |
| `id`         | `otelcol`        | A unique identifier for the operator. |
| `output`     | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `on_error`   | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`         |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `format`     | `auto`           | One of `auto`, `json`, `console`. Forces a specific zap encoding instead of detecting it per line. |

The schema is fixed - `ts`, `level`, `msg`, and `resource` always come from the same known shape zap's own encoders produce, so there is no field to configure it against. A log line either matches this shape or it isn't a collector self-log.

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
  "body": "{\"ts\":\"2026-07-06T22:56:21Z\",\"level\":\"warn\",\"msg\":\"Failed to scrape endpoint\",\"resource\":{\"k8s.pod.name\":\"otel-agent\"}}"
}
```

</td>
<td>

```json
{
  "timestamp": "2026-07-06T22:56:21Z",
  "severity": "warn",
  "body": "Failed to scrape endpoint",
  "resource": {
    "k8s.pod.name": "otel-agent"
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
  "body": "2026-07-06T22:56:21Z\twarn\ttx.go:127\tFailed to scrape endpoint\t{\"resource\":{\"k8s.pod.name\":\"otel-agent\"}}"
}
```

</td>
<td>

```json
{
  "timestamp": "2026-07-06T22:56:21Z",
  "severity": "warn",
  "body": "Failed to scrape endpoint",
  "resource": {
    "k8s.pod.name": "otel-agent"
  },
  "attributes": {
    "caller": "tx.go:127"
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
  "body": "{\"ts\":\"2026-07-06T22:56:21Z\",\"level\":\"info\",\"msg\":\"Starting server\"}"
}
```

</td>
<td>

```json
{
  "attributes": {
    "log.file.name": "collector.log"
  },
  "timestamp": "2026-07-06T22:56:21Z",
  "severity": "info",
  "body": "Starting server"
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
  "body": "{\"ts\":\"2026-07-06T22:56:21Z\",\"level\":\"info\",\"msg\":\"Starting server\"}"
}
```

</td>
<td>

```json
{
  "attributes": {
    "log.file.name": "app.log"
  },
  "body": "{\"ts\":\"2026-07-06T22:56:21Z\",\"level\":\"info\",\"msg\":\"Starting server\"}"
}
```

</td>
</tr>
</table>

#### Malformed trailing fields in a console-encoded line

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
  "body": "2026-07-06T22:56:21Z\twarn\tbroken message\t{\"resource\": invalid}"
}
```

</td>
<td>

```json
{
  "timestamp": "2026-07-06T22:56:21Z",
  "severity": "warn",
  "body": "broken message\t{\"resource\": invalid}",
  "attributes": {
    "otelcol.self_log.malformed_trailing_json": true
  }
}
```

</td>
</tr>
</table>

The line is not dropped or errored on - the raw trailing text is preserved as-is in `body`, and the attribute above signals that the loss is visible rather than silent. This is the only case where malformed input does not follow the operator's normal [on_error](../types/on_error.md) behavior; any other unparseable line (e.g. invalid full-line JSON in `json` mode, or a line that doesn't match the `console` shape at all) still triggers `on_error` as usual.