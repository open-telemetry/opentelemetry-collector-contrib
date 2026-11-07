# gnet Receiver

High-throughput OTLP receiver using the [gnet](https://github.com/panjf2000/gnet)
event-loop network library over raw TCP instead of the standard gRPC/HTTP2 stack.

## Wire protocol

Every frame:
```
[1 byte signal tag][4 bytes big-endian length][N bytes OTLP proto]
```

Signal tags: `0x01` = traces, `0x02` = metrics, `0x03` = logs.

## Configuration

```yaml
receivers:
  gnet:
    endpoint: "0.0.0.0:4320"   # TCP listen address
    num_event_loops: 0          # 0 = one per CPU core
    read_buffer_cap: 4096       # per-connection initial buffer (bytes)
```

## Use with gnet exporter

Pair with the `gnetexporter` on the sending side:

```yaml
# agent config.yaml
exporters:
  gnet:
    endpoint: "otel-gateway:4320"

# gateway config.yaml
receivers:
  gnet:
    endpoint: "0.0.0.0:4320"
```

## Status

Development — benchmarking in progress. Not production-ready.
