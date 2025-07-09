## `stdout` operator

The `stdout` operator will write entries to stdout in JSON format. This is particularly useful for debugging a config file
or running one-time batch processing jobs.

### Configuration Fields

| Field         | Default  | Description |
| ---           | ---      | ---         |
| `id`          | `stdout` | A unique identifier for the operator. |


### Example Configurations

#### Simple configuration

Configuration:
```yaml
- id: my_stdout
  type: stdout
```
