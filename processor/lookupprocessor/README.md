# Lookup Processor

| Status | |
| ------ | ----- |
| Stability | [development]: logs |
| Distributions | [] |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development

## Description

The lookup processor enriches telemetry signals by performing external lookups to retrieve additional data. It reads an attribute value, uses it as a key to query a lookup source, and sets the result as a new attribute.

Currently supports logs, with metrics and traces support planned.

## Configuration

```yaml
processors:
  lookup:
    source:
      type: yaml
      path: /etc/otel/mappings.yaml
    attributes:
      - key: user.name
        from_attribute: user.id
        default: "Unknown User"
        action: upsert
        context: record
```

### Full Configuration

| Field | Description | Default |
| ----- | ----------- | ------- |
| `source.type` | The source type identifier (`noop`, `yaml`) | `noop` |
| `attributes` | List of attribute enrichment rules (required) | - |

### Attribute Configuration

Each entry in `attributes` defines a lookup rule:

| Field | Description | Default |
| ----- | ----------- | ------- |
| `key` | Name of the attribute to set with the lookup result (required) | - |
| `from_attribute` | Name of the attribute containing the lookup key (required) | - |
| `default` | Value to use when lookup returns no result | - |
| `action` | How to handle the result: `insert`, `update`, `upsert` | `upsert` |
| `context` | Where to read/write attributes: `record`, `resource` | `record` |

### Actions

- **insert**: Only set the attribute if it doesn't already exist
- **update**: Only set the attribute if it already exists
- **upsert**: Always set the attribute (default)

### Context

- **record**: Read from and write to record-level attributes (log records, spans, metric data points) (default)
- **resource**: Read from and write to resource attributes

## Built-in Sources

### noop

A no-operation source that always returns "not found". Useful for testing.

```yaml
processors:
  lookup:
    source:
      type: noop
    attributes:
      - key: result
        from_attribute: key
        default: "not-found"
```

### yaml

Loads key-value mappings from a YAML file. The file should contain a flat map of string keys to values.

| Field | Description | Default |
| ----- | ----------- | ------- |
| `path` | Path to the YAML file (required) | - |

```yaml
processors:
  lookup:
    source:
      type: yaml
      path: /etc/otel/mappings.yaml
    attributes:
      - key: service.display_name
        from_attribute: service.name
```

Example mappings file (`mappings.yaml`):

```yaml
svc-frontend: "Frontend Web App"
svc-backend: "Backend API Service"
svc-worker: "Background Worker"
```

## Benchmarks

Run benchmarks with:

```bash
make benchmark
```

### Processor Performance

Measures the full processing pipeline including pdata operations, attribute iteration, value conversion, and telemetry. Uses noop source to isolate processor overhead from source implementation (Apple M4 Pro):

| Scenario | ns/op | B/op | allocs/op |
|----------|-------|------|-----------|
| 1 log, 1 attribute | 323 | 696 | 20 |
| 10 logs, 1 attribute | 1,274 | 3,216 | 74 |
| 100 logs, 1 attribute | 11,076 | 28,512 | 614 |
| 100 logs, 3 attributes | 21,604 | 54,113 | 1,014 |
| 1000 logs, 1 attribute | 122,447 | 280,617 | 6,014 |

### YAML Source Performance

Measures only the source lookup operation (map access), isolated from processor overhead:

| Map Size | ns/op | allocs/op |
|----------|-------|-----------|
| 10 entries | 1,418 | 0 |
| 100 entries | 1,355 | 0 |
| 1,000 entries | 1,317 | 0 |
| 10,000 entries | 1,319 | 0 |

## Custom Sources

Custom lookup sources can be added using `WithSources`:

```go
import (
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
    "github.com/example/httplookup"
)

factories.Processors[lookupprocessor.Type] = lookupprocessor.NewFactoryWithOptions(
    lookupprocessor.WithSources(httplookup.NewFactory()),
)
```

### Implementing a Source

```go
package mysource

import (
    "context"
    "errors"
    "time"

    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type Config struct {
    Endpoint string        `mapstructure:"endpoint"`
    Timeout  time.Duration `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
    if c.Endpoint == "" {
        return errors.New("endpoint is required")
    }
    return nil
}

func NewFactory() lookupsource.SourceFactory {
    return lookupsource.NewSourceFactory(
        "mysource",
        func() lookupsource.SourceConfig {
            return &Config{Timeout: 5 * time.Second}
        },
        createSource,
    )
}

func createSource(
    ctx context.Context,
    settings lookupsource.CreateSettings,
    cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
    c := cfg.(*Config)

    return lookupsource.NewSource(
        func(ctx context.Context, key string) (any, bool, error) {
            // Perform lookup - return (value, found, error)
            return "result", true, nil
        },
        func() string { return "mysource" },
        nil, // start function (optional)
        nil, // shutdown function (optional)
    ), nil
}
```

