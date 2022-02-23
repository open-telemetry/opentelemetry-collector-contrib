# Skywalking Receiver

Receives trace data in [Skywalking](https://skywalking.apache.org/) format.

Supported pipeline types: traces

## Getting Started

Examples:

```yaml
receivers:
  skywalking:
    protocols:
      grpc:
        endpoint: 0.0.0.0:11800
      http:
        endpoint: 0.0.0.0:12800

service:
  pipelines:
    traces:
      receivers: [skywalking]
```
