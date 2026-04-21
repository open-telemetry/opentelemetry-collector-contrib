## `stdin` operator

The `stdin` generates entries from lines written to stdin.

### Configuration Fields

| Field             | Default          | Description |
| ---               | ---              | ---         |
| `id`              | `stdin`          | A unique identifier for the operator. |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries. |

### Example Configurations

#### Mock a file input

Configuration:
```yaml
- type: stdin
```

Command:
```bash
echo "test" | stanza -c ./config.yaml
```

Output bodies:
```json
{
  "timestamp": "2020-11-10T11:09:56.505467-05:00",
  "severity": 0,
  "body": "test"
}
```
