# gnet Exporter

High-throughput OTLP exporter using persistent TCP with length-prefixed protobuf
frames, bypassing HTTP/2 and gRPC overhead. Pair with `gnetreceiver` on the
receiving side.

## Configuration

```yaml
exporters:
  gnet:
    endpoint: "otel-gateway:4320"  # gnetreceiver address
    dial_timeout: 5s
    write_timeout: 10s
    reconnect_delay: 1s
    max_reconnect_delay: 30s
```

## Status

Development — benchmarking in progress. Not production-ready.
