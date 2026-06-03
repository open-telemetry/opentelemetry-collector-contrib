# noop Source

A no-operation source that always returns "not-found". Useful for testing and benchmarking the processor overhead.

## Configuration

No configuration options.

## Example

```yaml
processors:
  lookup:
    source:
      type: noop
    lookups:
      - key: log.attributes["lookup.key"]
        attributes:
          - destination: result
            default: "not-found"
```
