# HAProxy Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The HAProxy receiver generates metrics by polling periodically the HAProxy process through a dedicated socket or HTTP URL.

Supported pipeline types: metrics

## Getting Started

## Configuration

### endpoint (required)
Path to the endpoint exposed by HAProxy for communications. It can be a local file socket or a HTTP URL.

### Collection interval settings (optional)
The scraping collection interval can be configured.

Default: 1 minute

### Example configuration

```yaml
haproxy:
  endpoint: file:///var/run/haproxy.ipc
  collection_interval: 1m
  metrics:
    
```

## Enabling metrics.

See [documentation.md](./documentation.md).

You can enable or disable selective metrics.

Example:

```yaml
receivers:
  haproxy:
    endpoint: http://127.0.0.1:8080/stats
    metrics:
      haproxy.connection_rate:
        enabled: false
      haproxy.requests:
        enabled: true
```

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha

