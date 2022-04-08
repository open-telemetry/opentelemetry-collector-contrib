# Skywalking Receiver

Receives trace data in [Skywalking](https://skywalking.apache.org/) format.

Supported pipeline types: traces

## ⚠️ Warning

Note: This component is experimental and is not recommended for production environments.

## Getting Started

By default, the Skywalking receiver will not serve any protocol. A protocol must be
named under the `protocols` object for the Skywalking receiver to start. The
below protocols are supported, each supports an optional `endpoint`
object configuration parameter.

- `grpc` (default `endpoint` = 0.0.0.0:11800)
- `http` (default `endpoint` = 0.0.0.0:12800)

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
