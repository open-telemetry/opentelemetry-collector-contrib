# yaml Source

Looks up keys from a YAML file loaded at startup. Supports both scalar values (1:1 mapping) and nested maps (1:N mapping). The file is read once during `Start` and kept in memory.

## Configuration

| Field | Description | Default |
| ----- | ----------- | ------- |
| `path` | Path to the YAML file containing key-value mappings (required) | - |

## Examples

### Scalar values (1:1)

YAML file:

```yaml
user001: "Alice Johnson"
user002: "Bob Smith"
```

Processor config:

```yaml
processors:
  lookup:
    source:
      type: yaml
      path: /etc/otel/mappings.yaml
    lookups:
      - key: log.attributes["user.id"]
        attributes:
          - destination: user.name
            default: "Unknown User"
```

### Nested maps (1:N)

YAML file:

```yaml
user001:
  name: "Alice Johnson"
  email: "alice@example.com"
  role: "admin"
user002:
  name: "Bob Smith"
  email: "bob@example.com"
  role: "viewer"
```

Processor config:

```yaml
processors:
  lookup:
    source:
      type: yaml
      path: /etc/otel/user-details.yaml
    lookups:
      - key: log.attributes["user.id"]
        attributes:
          - source: name
            destination: user.name
          - source: email
            destination: user.email
          - source: role
            destination: user.role
```

## Benchmarks

Run with `go test -bench=. ./internal/source/yaml/`

Lookup performance across different map sizes (Apple M4 Pro):

| Map Size | ns/op | allocs/op |
|----------|-------|-----------|
| 10 entries | 1,462 | 0 |
| 100 entries | 1,415 | 0 |
| 1,000 entries | 1,388 | 0 |
| 10,000 entries | 1,368 | 0 |

Lookup time is constant regardless of map size (Go map access is O(1)), with zero allocations.
