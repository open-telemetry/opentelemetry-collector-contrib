# Entry

Entry is the base representation of log data as it moves through a pipeline. All operators either create, modify, or consume entries.

## Structure
| Field            | Description |
| ---              | ---         |
| `timestamp`      | The timestamp associated with the log (RFC 3339). |
| `severity`       | The [severity](../types/field.md) of the log. |
| `severity_text`  | The original text that was interpreted as a [severity](../types/field.md). |
| `resource`       | A map of key/value pairs that describe the resource from which the log originated. |
| `attributes`     | A map of key/value pairs that provide additional context to the log. This value is often used by a consumer to filter logs. |
| `body`           | The contents of the log. This value is often modified and restructured in the pipeline. It may be a string, number, or object. |


Represented in `json` format, an entry may look like the following:

```json
{
  "resource": {
    "uuid": "11112222-3333-4444-5555-666677778888",
  },
  "attributes": {
    "env": "prod",
  },
  "body": {
    "message": "Something happened.",
    "details": {
      "count": 100,
      "reason": "event",
    },
  },
  "timestamp": "2020-01-31T00:00:00-00:00",
  "severity": 30,
  "severity_text": "INFO",
}
```

Throughout the documentation, `json` format is used to represent entries. Fields are typically omitted unless relevant to the behavior being described.
