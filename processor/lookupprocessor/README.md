# Lookup Processor (skeleton)

The lookup processor enriches telemetry by delegating key lookups to an external lookup extension. The processor itself does not ship a built-in source; configure any lookup extension (for example the noop lookup extension from this branch or future YAML/HTTP/Memcached extensions) and reference its `component.ID` under `source.extension`.

## Minimal configuration

```yaml
extensions:
  nooplookup: {}

processors:
  lookup:
    source:
      extension: nooplookup
```

The sample mirrors the contents of `testdata/config.yaml` and demonstrates how to associate the processor with an extension instance registered on the host.
