# Lookup Processor

| Status | |
| ------ | ----- |
| Stability | [development]: logs |
| Distributions | [] |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development

## Description

The lookup processor enriches telemetry signals by performing external lookups to retrieve additional data. Currently supports logs, with metrics and traces support planned.

Lookup sources are built into the collector and can be extended through the `WithSources` factory option. Sources can optionally use caching and timeouts to improve performance and reliability.

## Configuration

```yaml
processors:
  lookup:
    source:
      type: noop  # Source type identifier
      # Source-specific configuration goes here
```

### Source Configuration

The `source` block configures which lookup source to use:

| Field | Description | Default |
| ----- | ----------- | ------- |
| `type` | The source type identifier (e.g., `noop`, `yaml`, `dns`) | `noop` |

Additional fields depend on the specific source type being used.

## Built-in Sources

### noop

A no-operation source that always returns "not found". Useful for testing.

```yaml
processors:
  lookup:
    source:
      type: noop
```

## Caching

Sources can use the built-in caching support via `lookupsource.WrapWithCache`:

| Field | Description | Default |
| ----- | ----------- | ------- |
| `cache.enabled` | Enable caching | `false` |
| `cache.size` | Maximum number of entries | `1000` |
| `cache.ttl` | Time-to-live for cached entries | `0` (no expiration) |

## Custom Sources

Custom lookup sources can be added to the processor using `WithSources`:

```go
import (
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
    "github.com/user/otel-lookup-http/httplookup"
)

// Replace default sources with custom ones
factories.Processors[lookupprocessor.Type] = lookupprocessor.NewFactoryWithOptions(
    lookupprocessor.WithSources(httplookup.NewFactory()),
)
```

### Implementing a Source

To create a custom source, implement the `lookupsource.SourceFactory` interface:

```go
package mysource

import (
    "context"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type Config struct {
    Endpoint string        `mapstructure:"endpoint"`
    Timeout  time.Duration `mapstructure:"timeout"`
    Cache    lookupsource.CacheConfig `mapstructure:"cache"`
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
            return &Config{
                Timeout: 5 * time.Second,
            }
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

    lookupFn := func(ctx context.Context, key string) (any, bool, error) {
        // Perform lookup logic here
        return value, true, nil
    }

    // Optionally wrap with caching
    cache := lookupsource.NewCache(c.Cache)
    cachedLookup := lookupsource.WrapWithCache(cache, lookupFn)

    return lookupsource.NewSource(
        cachedLookup,
        func() string { return "mysource" },
        nil, // start function (optional)
        nil, // shutdown function (optional)
    ), nil
}
```
